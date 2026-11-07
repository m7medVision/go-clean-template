package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/m7medvision/go-clean-template/internal/repo/webapi"
	"google.golang.org/grpc"
)

const (
	_readTimeout = 5 * time.Second
	// _writeMargin is how much longer than its waits a response may take: the slowest write still has
	// to save and answer once it has its rate and its projections.
	_writeMargin = 3 * time.Second
	// _shutdownTimeout bounds how long each server finishes the requests and streams in flight once it
	// is told to stop; it drops what is left after that.
	_shutdownTimeout = 3 * time.Second
)

// server is what listenAndServe runs and stops: an HTTP or a gRPC server.
type server struct {
	name, addr string
	serve      func(net.Listener) error
	// shutdown stops taking work and returns once the work in flight has finished, or with ctx's
	// error when ctx ends first.
	shutdown func(ctx context.Context) error
	// stopNow drops the work still in flight.
	stopNow func()
}

// newHTTPServer serves h on port. The slowest write, a conversion, waits up to webapi.Timeout for
// its rate and then up to consistencyTimeout for its projections, so the write deadline is both
// plus a margin: a shorter one would cut off the answer of a write that waits its full time.
func newHTTPServer(port string, h http.Handler, consistencyTimeout time.Duration) *http.Server {
	return &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      h,
		ReadTimeout:  _readTimeout,
		WriteTimeout: webapi.Timeout + consistencyTimeout + _writeMargin,
	}
}

// serveHTTP serves srv until ctx ends, then gives the requests in flight drain to finish.
func serveHTTP(ctx context.Context, srv *http.Server, drain time.Duration, l *slog.Logger) error {
	return listenAndServe(ctx, server{
		name: "http", addr: srv.Addr, serve: srv.Serve, shutdown: srv.Shutdown,
		// Close's only error is its listener's, which Shutdown has already closed.
		stopNow: func() { _ = srv.Close() },
	}, drain, l)
}

// serveGRPC serves srv on port until ctx ends, then gives the calls and streams in flight drain to
// finish.
func serveGRPC(ctx context.Context, srv *grpc.Server, port string, drain time.Duration, l *slog.Logger) error {
	return listenAndServe(ctx, server{
		name: "grpc", addr: net.JoinHostPort("", port), serve: srv.Serve,
		shutdown: func(ctx context.Context) error { return gracefulStop(ctx, srv) },
		stopNow:  srv.Stop,
	}, drain, l)
}

// gracefulStop is GracefulStop bounded by ctx. GracefulStop waits for every open stream, and a
// client such as grpcui holds a reflection stream open for as long as it runs. The Stop that
// follows a timeout ends the GracefulStop still running.
func gracefulStop(ctx context.Context, srv *grpc.Server) error {
	stopped := make(chan struct{})

	go func() {
		srv.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("grpc GracefulStop: %w", ctx.Err())
	}
}

func listenAndServe(ctx context.Context, s server, drain time.Duration, l *slog.Logger) error {
	var lc net.ListenConfig

	// Listening is immediate, so it ignores ctx: a server stopped before it started still stops cleanly.
	ln, err := lc.Listen(context.WithoutCancel(ctx), "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("%s server - listen: %w", s.name, err)
	}

	l.Info("server started", "server", s.name, "address", ln.Addr().String())

	served := make(chan error, 1)

	go func() { served <- s.serve(ln) }()

	select {
	case err = <-served:
		return fmt.Errorf("%s server - serve: %w", s.name, err)
	case <-ctx.Done():
	}

	if err = stop(context.WithoutCancel(ctx), s, drain, l); err != nil {
		return fmt.Errorf("%s server - stop: %w", s.name, err)
	}

	// A gRPC server stopped before its Serve began answers ErrServerStopped, an HTTP one ErrServerClosed.
	if err = <-served; err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("%s server - serve: %w", s.name, err)
	}

	l.Info("server stopped", "server", s.name)

	return nil
}

// stop gives s drain to finish the work in flight, then drops what is left: a signaled stop is a
// clean stop, however long a request or a stream would still have run.
func stop(ctx context.Context, s server, drain time.Duration, l *slog.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, drain)
	defer cancel()

	err := s.shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	l.Warn("server dropped the work still in flight after the drain", "server", s.name, "drain", drain.String())
	s.stopNow()

	return nil
}
