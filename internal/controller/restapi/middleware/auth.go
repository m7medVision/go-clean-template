// Package middleware is REST's own middleware: Auth, the Huma middleware that enforces each
// operation's Security. What wraps every HTTP request is in internal/controller/httpmw.
package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/httpmw"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

const _bearerParts = 2

// Auth returns the Huma middleware that enforces each operation's declared Security. An operation
// that declares a requirement runs with the caller of the request's bearer token, which its handler
// reads with authn.MustCaller, or answers 401 when the token is missing or invalid; one that
// declares none is public. The OpenAPI document states the same Security, so what the docs say
// needs a token and what the server enforces are one declaration, and an operation is protected
// whatever its path or version. A request no operation matched never reaches it: it is a 404.
func Auth(tokens *jwt.Manager, l *slog.Logger) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if len(ctx.Operation().Security) == 0 {
			next(ctx)

			return
		}

		c, failure := readCaller(tokens, ctx.Header("Authorization"))
		if failure != "" {
			_, w := humachi.Unwrap(ctx)
			httpmw.WriteProblem(l, w, http.StatusUnauthorized, failure)

			return
		}

		next(huma.WithContext(ctx, authn.WithCaller(ctx.Context(), c)))
	}
}

func readCaller(tokens *jwt.Manager, header string) (c authn.Caller, failure string) {
	if header == "" {
		return authn.Caller{}, "missing authorization header"
	}

	parts := strings.SplitN(header, " ", _bearerParts)
	if len(parts) != _bearerParts || parts[0] != "Bearer" {
		return authn.Caller{}, "invalid authorization header format"
	}

	c, err := authn.ReadToken(tokens, parts[1])
	if err != nil {
		return authn.Caller{}, "invalid or expired token"
	}

	return c, ""
}
