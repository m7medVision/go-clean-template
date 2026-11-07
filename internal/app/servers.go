package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
)

const (
	_readTimeout = 5 * time.Second
	// _writeMargin is how much longer than the consistency timeout a response may take: a write
	// waits that long for its projections, then still has to answer.
	_writeMargin = 3 * time.Second
	// _shutdownTimeout bounds how long the HTTP server finishes the requests in flight.
	_shutdownTimeout = 3 * time.Second
)

// newHTTPServer serves h on port. A write waits up to consistencyTimeout for its projections, so
// the write deadline is that plus a margin: a fixed one would cut off the answer of any write that
// waits longer.
func newHTTPServer(port string, h http.Handler, consistencyTimeout time.Duration) *http.Server {
	return &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      h,
		ReadTimeout:  _readTimeout,
		WriteTimeout: consistencyTimeout + _writeMargin,
	}
}

// serveHTTP serves srv until ctx ends, then shuts it down, letting the requests in flight finish
// for _shutdownTimeout. It fails when srv cannot listen or stops serving on its own.
func serveHTTP(ctx context.Context, srv *http.Server, l *slog.Logger) error {
	return listenAndServe(ctx, "http", srv.Addr, l, srv.Serve, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, _shutdownTimeout)
		defer cancel()

		return srv.Shutdown(ctx)
	})
}

// serveGRPC serves srv on port until ctx ends, then stops it once the calls in flight finish.
func serveGRPC(ctx context.Context, srv *grpc.Server, port string, l *slog.Logger) error {
	return listenAndServe(ctx, "grpc", net.JoinHostPort("", port), l, srv.Serve, func(context.Context) error {
		srv.GracefulStop()

		return nil
	})
}

// listenAndServe listens on addr and runs serve on the listener until ctx ends; then it runs stop
// and waits for serve to return.
func listenAndServe(
	ctx context.Context, name, addr string, l *slog.Logger,
	serve func(net.Listener) error, stop func(context.Context) error,
) error {
	var lc net.ListenConfig

	// Listening is immediate, so it ignores ctx: a server stopped before it started still stops cleanly.
	ln, err := lc.Listen(context.WithoutCancel(ctx), "tcp", addr)
	if err != nil {
		return fmt.Errorf("%s server - listen: %w", name, err)
	}

	l.Info("server started", "server", name, "address", ln.Addr().String())

	served := make(chan error, 1)

	go func() { served <- serve(ln) }()

	select {
	case err = <-served:
		return fmt.Errorf("%s server - serve: %w", name, err)
	case <-ctx.Done():
	}

	if err = stop(context.WithoutCancel(ctx)); err != nil {
		return fmt.Errorf("%s server - stop: %w", name, err)
	}

	if err = <-served; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("%s server - serve: %w", name, err)
	}

	l.Info("server stopped", "server", name)

	return nil
}
