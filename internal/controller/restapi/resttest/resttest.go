// Package resttest serves the real REST router for the tests of the restapi package and of each v1
// domain folder: it is to restapi what net/http/httptest is to net/http.
package resttest

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/require"
)

// _signingKey is the JWT_SECRET Main sets, so the tokens Token and AdminToken sign are valid.
const _signingKey = "test-secret"

// Main sets the environment config.NewConfig reads, then runs the tests. Call it from TestMain.
func Main(m *testing.M) {
	for _, env := range []struct{ key, value string }{
		{"APP_NAME", "test"},
		{"APP_VERSION", "1.0.0"},
		{"HTTP_PORT", "0"},
		{"LOG_LEVEL", "info"},
		{"PG_POOL_MAX", "1"},
		{"PG_URL", "postgres://localhost/db"},

		{"JWT_SECRET", _signingKey},
		{"DOCS_ENABLED", "true"},
	} {
		if err := os.Setenv(env.key, env.value); err != nil {
			panic(err)
		}
	}

	os.Exit(m.Run())
}

// Config returns the configuration Main's environment describes.
func Config(t *testing.T) *config.Config {
	t.Helper()

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	return cfg
}

// NewRouter serves the REST router over the given buses, plus each option's operations. A bus the
// test never reaches may be nil.
func NewRouter(cfg *config.Config, b cqrs.Buses, opts ...restapi.Option) http.Handler {
	jwtManager := jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry)

	return restapi.NewRouter(cfg, b, jwtManager, slog.New(slog.DiscardHandler), opts...)
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
