package middleware

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const principalKey contextKey = "principal"

// UserIDFromContext extracts the user ID from the context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	principal, ok := PrincipalFromContext(ctx)

	return principal.UserID, ok
}

// PrincipalFromContext extracts the caller and the roles their token grants from the context.
func PrincipalFromContext(ctx context.Context) (entity.Principal, bool) {
	principal, ok := ctx.Value(principalKey).(entity.Principal)

	return principal, ok
}

// AuthInterceptor returns a gRPC unary interceptor for JWT authentication.
func AuthInterceptor(jwtManager *jwt.Manager) grpc.UnaryServerInterceptor {
	skipAuthMethods := map[string]bool{
		"/grpc.v1.AuthService/Register": true,
		"/grpc.v1.AuthService/Login":    true,
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if skipAuthMethods[info.FullMethod] {
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

		claims, err := jwtManager.Parse(values[0])
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}

		ctx = context.WithValue(ctx, principalKey, entity.Principal{
			UserID: claims.UserID,
			Roles:  entity.ParseRoles(claims.Roles),
		})

		return handler(ctx, req)
	}
}
