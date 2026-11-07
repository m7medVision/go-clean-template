package app

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/repo/webapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const _stopWait = 5 * time.Second

// serveUntilCanceled runs serve on a context it cancels once the server answers, and returns what
// serve returned.
func serveUntilCanceled(t *testing.T, serve func(ctx context.Context) error, ready func() bool) error {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() { done <- serve(ctx) }()

	require.Eventually(t, ready, _stopWait, 10*time.Millisecond)
	cancel()

	select {
	case err := <-done:
		return err
	case <-time.After(_stopWait):
		t.Fatal("the server did not stop")

		return nil
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

	err := serveUntilCanceled(t, func(ctx context.Context) error {
		return serveHTTP(ctx, srv, slog.New(slog.DiscardHandler))
	}, dials(port))

	require.NoError(t, err)
}

func TestServeGRPCStopsCleanlyWhenTheContextEnds(t *testing.T) {
	t.Parallel()

	port := freePort(t)

	err := serveUntilCanceled(t, func(ctx context.Context) error {
		return serveGRPC(ctx, grpc.NewServer(), port, slog.New(slog.DiscardHandler))
	}, dials(port))

	require.NoError(t, err)
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

	require.Error(t, serveHTTP(t.Context(), srv, slog.New(slog.DiscardHandler)))
	require.Error(t, serveGRPC(t.Context(), grpc.NewServer(), port, slog.New(slog.DiscardHandler)))
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
