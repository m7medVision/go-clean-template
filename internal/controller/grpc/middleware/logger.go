package middleware

import (
	"context"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Logger returns the interceptor that logs the client address, method and status code of every
// unary call when it ends.
func Logger(l *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)

		l.Info("grpc request", "client", clientAddr(ctx), "method", info.FullMethod, "code", status.Code(err).String())

		return resp, err
	}
}

// clientAddr is the host the call came from, or empty when gRPC does not know it.
func clientAddr(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ""
	}

	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return p.Addr.String()
	}

	return host
}
