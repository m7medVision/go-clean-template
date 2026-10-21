package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

const (
	_apiPrefix        = "/v1/"
	_authPrefix       = "/v1/auth/"
	_bearerParts      = 2
	_problemTypeBlank = "about:blank"
)

type authContextKey struct{}

// caller is what Auth stores in the request context.
type caller struct {
	userID   string
	issuedAt time.Time
}

type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

// UserID returns the caller identifier stored in the request context by Auth.
func UserID(ctx context.Context) (string, bool) {
	c, ok := ctx.Value(authContextKey{}).(caller)

	return c.userID, ok
}

// AuthTime returns when the caller's token was issued, stored in the request context by Auth: the
// last time they signed in. It is zero for a token issued without one.
func AuthTime(ctx context.Context) (time.Time, bool) {
	c, ok := ctx.Value(authContextKey{}).(caller)

	return c.issuedAt, ok
}

// Auth authenticates the request and stores the caller identifier in the request
// context. Routes outside /v1 (docs, health, metrics) and the auth endpoints
// under /v1/auth are public.
func Auth(jwtManager *jwt.Manager, l logger.Interface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			public := !strings.HasPrefix(r.URL.Path, _apiPrefix) || strings.HasPrefix(r.URL.Path, _authPrefix)
			if public {
				next.ServeHTTP(w, r)

				return
			}

			header := r.Header.Get("Authorization")
			if header == "" {
				writeProblem(l, w, http.StatusUnauthorized, "missing authorization header")

				return
			}

			parts := strings.SplitN(header, " ", _bearerParts)
			if len(parts) != _bearerParts || parts[0] != "Bearer" {
				writeProblem(l, w, http.StatusUnauthorized, "invalid authorization header format")

				return
			}

			claims, err := jwtManager.Parse(parts[1])
			if err != nil {
				writeProblem(l, w, http.StatusUnauthorized, "invalid or expired token")

				return
			}

			ctx := context.WithValue(r.Context(), authContextKey{}, caller{userID: claims.UserID, issuedAt: claims.IssuedAt})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeProblem writes an RFC 9457 problem response on the auth failures.
func writeProblem(l logger.Interface, w http.ResponseWriter, status int, detail string) {
	body, err := json.Marshal(problem{
		Type:   _problemTypeBlank,
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	})
	if err != nil {
		l.Error(err, "restapi - middleware - writeProblem")

		return
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	if _, err := w.Write(body); err != nil {
		l.Error(err, "restapi - middleware - writeProblem")
	}
}
