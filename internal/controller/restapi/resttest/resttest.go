// Package resttest serves the real REST handler for the tests of the restapi package and of each v1
// domain folder: it is to restapi what net/http/httptest is to net/http.
package resttest

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/require"
)

// _signingKey signs the tokens Token and AdminToken return, and the router checks them with it.
const _signingKey = "test-secret"

// Routes registers a domain folder's operations: its NewRoutes.
type Routes func(api huma.API, b cqrs.Buses, l *slog.Logger)

// NewRouter serves the REST handler with each routes' operations over the given buses, and its
// docs, behind what the app puts in front of it: trailing slashes stripped and panics recovered. A
// bus the test never reaches may be nil.
func NewRouter(b cqrs.Buses, routes ...Routes) http.Handler {
	l := slog.New(slog.DiscardHandler)

	rest := restapi.NewHandler("1.0.0", true, jwt.New(_signingKey, time.Hour), l, func(api huma.API) {
		for _, register := range routes {
			register(api, b, l)
		}
	})

	r := chi.NewRouter()
	r.Use(chimw.StripSlashes, middleware.Recovery(l))
	r.Mount("/", rest)

	return r
}

// NewBuses returns empty command and query buses with no pipeline steps, for a test to register
// its fakes' handlers on.
func NewBuses() cqrs.Buses {
	return cqrs.Buses{Commands: cqrs.NewCommands(), Queries: cqrs.NewQueries()}
}

// Token signs a token for user-1 with no roles.
func Token(t *testing.T) string {
	t.Helper()

	token, err := jwt.New(_signingKey, time.Hour).GenerateToken("user-1", nil)
	require.NoError(t, err)

	return token
}

// AdminToken signs a token for admin-1 with the admin and user roles.
func AdminToken(t *testing.T) string {
	t.Helper()

	token, err := jwt.New(_signingKey, time.Hour).GenerateToken("admin-1", []string{"admin", "user"})
	require.NoError(t, err)

	return token
}

// Do sends body to target, with token as the bearer token unless it is empty.
func Do(ctx context.Context, handler http.Handler, method, target, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	return resp
}
