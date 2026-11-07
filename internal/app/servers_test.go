package app

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/repo/webapi"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
)

const (
	_stopWait = 5 * time.Second
	// _testDrain stands in for _shutdownTimeout, so a test that outlasts it stays short.
	_testDrain = 100 * time.Millisecond
	// _dropped is what a server logs when it drops the work still in flight.
	_dropped = "server dropped the work still in flight after the drain"
)

// startServer runs serve on a context of its own until the server answers on port. stop ends that
// context and returns what serve returned, failing the test when the server does not stop.
func startServer(t *testing.T, port string, serve func(ctx context.Context) error) (stop func() error) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() { done <- serve(ctx) }()

	require.Eventually(t, dials(port), _stopWait, 10*time.Millisecond)

	return func() error {
		cancel()

		select {
		case err := <-done:
			return err
		case <-time.After(_stopWait):
			t.Fatal("the server did not stop")

			return nil
		}
	}
}

// freePort returns a port nothing listens on, for a server under test to take.
func freePort(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	require.NoError(t, ln.Close())

	return port
}

func dials(port string) func() bool {
	return func() bool {
		var d net.Dialer

		conn, err := d.DialContext(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", port))
		if err != nil {
			return false
		}

		_ = conn.Close()

		return true
	}
}

func TestServeHTTPStopsCleanlyWhenTheContextEnds(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	srv := newHTTPServer(port, http.NotFoundHandler(), time.Second)

	stop := startServer(t, port, func(ctx context.Context) error {
		return serveHTTP(ctx, srv, _testDrain, slog.New(slog.DiscardHandler))
	})

	require.NoError(t, stop())
}

func TestServeGRPCStopsCleanlyWhenTheContextEnds(t *testing.T) {
	t.Parallel()

	port := freePort(t)

	stop := startServer(t, port, func(ctx context.Context) error {
		return serveGRPC(ctx, grpc.NewServer(), port, _testDrain, slog.New(slog.DiscardHandler))
	})

	require.NoError(t, stop())
}

// A request still running after the drain is dropped, and the stop is still a clean one: a SIGTERM
// does not fail the app because a request was slow.
func TestServeHTTPDropsARequestThatOutlastsTheDrain(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	started := make(chan struct{}, 1)
	srv := newHTTPServer(port, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		started <- struct{}{}

		<-r.Context().Done()
	}), time.Second)

	var logs bytes.Buffer

	stop := startServer(t, port, func(ctx context.Context) error {
		return serveHTTP(ctx, srv, _testDrain, slog.New(slog.NewJSONHandler(&logs, nil)))
	})

	go func() {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+net.JoinHostPort("127.0.0.1", port), http.NoBody)
		if err != nil {
			return
		}

		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}()

	select {
	case <-started:
	case <-time.After(_stopWait):
		t.Fatal("the request never reached the handler")
	}

	require.NoError(t, stop())
	assert.Contains(t, logs.String(), _dropped)
	assert.Contains(t, logs.String(), `"server":"http"`)
}

// The app's gRPC server serves reflection, whose stream a client such as grpcui holds open for as
// long as it runs: the server still stops once the drain is over, and cleanly.
func TestServeGRPCDropsAStreamThatOutlastsTheDrain(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	srv := newGRPCServer(newTestBuses(), jwt.New("test-secret", time.Hour), newMetrics(), true, slog.New(slog.DiscardHandler))

	var logs bytes.Buffer

	stop := startServer(t, port, func(ctx context.Context) error {
		return serveGRPC(ctx, srv, port, _testDrain, slog.New(slog.NewJSONHandler(&logs, nil)))
	})

	conn, err := grpc.NewClient(net.JoinHostPort("127.0.0.1", port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	stream, err := reflectionpb.NewServerReflectionClient(conn).ServerReflectionInfo(t.Context())
	require.NoError(t, err)
	require.NoError(t, stream.Send(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{},
	}))

	_, err = stream.Recv()
	require.NoError(t, err, "the stream is open")

	require.NoError(t, stop())
	assert.Contains(t, logs.String(), _dropped)
	assert.Contains(t, logs.String(), `"server":"grpc"`)
}

// A server that cannot listen fails at once, without waiting for the context to end, so the
// errgroup stops the other servers.
func TestServeFailsWhenItCannotListen(t *testing.T) {
	t.Parallel()

	var lc net.ListenConfig

	taken, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = taken.Close() })

	_, port, err := net.SplitHostPort(taken.Addr().String())
	require.NoError(t, err)

	srv := newHTTPServer(port, http.NotFoundHandler(), time.Second)
	srv.Addr = taken.Addr().String()

	require.Error(t, serveHTTP(t.Context(), srv, _testDrain, slog.New(slog.DiscardHandler)))
	require.Error(t, serveGRPC(t.Context(), grpc.NewServer(), port, _testDrain, slog.New(slog.DiscardHandler)))
}

// A conversion waits up to the rate API's timeout for its rate, then up to the consistency timeout
// for its projections, so its answer must still fit in the write deadline.
func TestTheWriteDeadlineOutlastsTheSlowestWrite(t *testing.T) {
	t.Parallel()

	for _, timeout := range []time.Duration{time.Second, 2 * time.Second, time.Minute} {
		srv := newHTTPServer("0", http.NotFoundHandler(), timeout)
		assert.Greater(t, srv.WriteTimeout, webapi.Timeout+timeout)
	}
}
