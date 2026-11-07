// Package middleware reads the caller of a GraphQL request from its bearer token.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

// Caller returns the signed-in caller, or the UNAUTHENTICATED error a resolver returns as it is.
// GraphQL lets a request without a token through, so every resolver that needs a caller asks here.
func Caller(ctx context.Context) (authn.Caller, error) {
	c, ok := authn.CallerFrom(ctx)
	if !ok {
		return authn.Caller{}, gqlerr.Unauthenticated()
	}

	return c, nil
}

// WithOptionalCaller stores the caller of a valid bearer token in the request context and lets
// every request through, so introspection works without a token. Resolvers check for a caller with
// Caller.
func WithOptionalCaller(jwtManager *jwt.Manager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			if c, err := authn.ReadToken(jwtManager, token); err == nil {
				r = r.WithContext(authn.WithCaller(r.Context(), c))
			}
		}

		next.ServeHTTP(w, r)
	})
}
