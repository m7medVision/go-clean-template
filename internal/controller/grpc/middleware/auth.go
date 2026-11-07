// Package middleware holds the gRPC server's interceptors, in the order the server chains them: the
// request log, metrics, panic recovery and the token check.
package middleware

import (
	"context"
	"slices"

	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Auth returns the interceptor that runs every unary call with the caller of the bare token in its
// authorization metadata, which the method reads with authn.MustCaller, or answers Unauthenticated
// when the token is missing or invalid. The methods in public, named by their generated
// FullMethodName constants, are called without a token.
func Auth(tokens *jwt.Manager, public ...string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if slices.Contains(public, info.FullMethod) {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization token")
		}

		c, err := authn.ReadToken(tokens, values[0])
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}

		return handler(authn.WithCaller(ctx, c), req)
	}
}
