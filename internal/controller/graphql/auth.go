package graphql

import (
	"context"
	"net/http"
	"strings"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

type authContextKey struct{}

// UserID returns the caller identifier stored in the request context by auth.
func UserID(ctx context.Context) (string, bool) {
	principal, ok := Principal(ctx)

	return principal.UserID, ok
}

// Principal returns the caller and the roles their token grants, stored in the request context by
// auth.
func Principal(ctx context.Context) (entity.Principal, bool) {
	principal, ok := ctx.Value(authContextKey{}).(entity.Principal)

	return principal, ok
}

func withOptionalCaller(jwtManager *jwt.Manager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			if claims, err := jwtManager.Parse(token); err == nil {
				principal := entity.Principal{UserID: claims.UserID, Roles: entity.ParseRoles(claims.Roles)}
				r = r.WithContext(context.WithValue(r.Context(), authContextKey{}, principal))
			}
		}

		next.ServeHTTP(w, r)
	})
}
