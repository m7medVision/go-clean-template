// Package resttest serves the real REST router for the tests of the restapi package and of each v1
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

	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/require"
)

// _signingKey signs the tokens Token and AdminToken return, and the router checks them with it.
const _signingKey = "test-secret"

// NewRouter serves the REST router over the given buses, plus each option's operations, with the
// docs and metrics on. A bus the test never reaches may be nil.
func NewRouter(b cqrs.Buses, opts ...restapi.Option) http.Handler {
	s := restapi.Settings{Version: "1.0.0", Docs: true, Metrics: true}

	return restapi.NewRouter(s, b, jwt.New(_signingKey, time.Hour), slog.New(slog.DiscardHandler), opts...)
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
