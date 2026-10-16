package graphql

import (
	"context"
	"net/http"
	"strings"

	"github.com/evrone/go-clean-template/pkg/jwt"
)

type authContextKey struct{}

// UserID returns the caller identifier stored in the request context by auth.
func UserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(authContextKey{}).(string)

	return userID, ok
}

func withOptionalCaller(jwtManager *jwt.Manager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			if userID, err := jwtManager.ParseToken(token); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), authContextKey{}, userID))
			}
		}

		next.ServeHTTP(w, r)
	})
}
