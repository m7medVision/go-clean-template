// Package middleware reads the caller of a GraphQL request from its bearer token.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

type authContextKey struct{}

// Principal returns the caller and the roles their token grants, stored in the request context by
// WithOptionalCaller.
func Principal(ctx context.Context) (entity.Principal, bool) {
	principal, ok := ctx.Value(authContextKey{}).(entity.Principal)

	return principal, ok
}

// Caller returns the signed-in caller, or the UNAUTHENTICATED error a resolver returns as it is.
func Caller(ctx context.Context) (entity.Principal, error) {
	principal, ok := Principal(ctx)
	if !ok {
		return entity.Principal{}, gqlerr.Unauthenticated()
	}

	return principal, nil
}

// WithOptionalCaller stores the caller of a valid bearer token in the request context and lets
// every request through, so introspection works without a token. Resolvers check for a caller with
// Caller.
func WithOptionalCaller(jwtManager *jwt.Manager, next http.Handler) http.Handler {
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
