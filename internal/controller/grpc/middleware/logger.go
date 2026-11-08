package middleware

import (
	"context"
	"log/slog"
	"net"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func Logger(l *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)

		l.Info("grpc request", "client", clientAddr(ctx), "method", info.FullMethod, "code", status.Code(err).String())

		return resp, err
	}
}

func clientAddr(ctx context.Context) string {
	if client, ok := entity.ClientFromContext(ctx); ok {
		return client.String()
	}

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
