package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Recovery returns the interceptor that turns a panic in a unary call into Internal, and logs the
// panic with its stack trace. Without it, gRPC lets the panic end the process.
func Recovery(l *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if p := recover(); p != nil {
				resp, err = nil, panicked(ctx, l, info.FullMethod, p)
			}
		}()

		return handler(ctx, req)
	}
}

// StreamRecovery is Recovery for a streaming call, such as server reflection's.
func StreamRecovery(l *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if p := recover(); p != nil {
				err = panicked(ss.Context(), l, info.FullMethod, p)
			}
		}()

		return handler(srv, ss)
	}
}

func panicked(ctx context.Context, l *slog.Logger, method string, p any) error {
	l.Error("grpc panic", "client", clientAddr(ctx), "method", method,
		"panic", fmt.Sprint(p), "stack", string(debug.Stack()))

	return status.Error(codes.Internal, "internal server error")
}
