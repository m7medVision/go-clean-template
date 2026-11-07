package middleware_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type callerOutput struct {
	Body struct {
		UserID    string        `json:"user_id"`
		Roles     []entity.Role `json:"roles"`
		IssuedAt  time.Time     `json:"issued_at"`
		HasCaller bool          `json:"has_caller"`
	}
}

// echoCaller answers with the caller the request carries, if any.
func echoCaller(ctx context.Context, _ *struct{}) (*callerOutput, error) {
	out := &callerOutput{}

	c, ok := authn.CallerFrom(ctx)
	out.Body.UserID, out.Body.Roles, out.Body.IssuedAt, out.Body.HasCaller = c.UserID, c.Roles, c.IssuedAt, ok

	return out, nil
}

// newTestHandler serves GET /private, which declares Security, and GET /public, which does not,
// behind Auth.
func newTestHandler(tokens *jwt.Manager) http.Handler {
	r := chi.NewRouter()
	api := humachi.New(r, huma.DefaultConfig("test", "1.0.0"))
	api.UseMiddleware(middleware.Auth(tokens, slog.New(slog.DiscardHandler)))

	huma.Register(api, huma.Operation{
		OperationID: "private", Method: http.MethodGet, Path: "/private",
		Security: []map[string][]string{{"BearerAuth": {}}},
	}, echoCaller)
	huma.Register(api, huma.Operation{OperationID: "public", Method: http.MethodGet, Path: "/public"}, echoCaller)

	return r
}

func get(t *testing.T, h http.Handler, target, authorization string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	return resp
}

func TestAuthRejectsAnOperationThatDeclaresSecurity(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(jwt.New("test-secret", time.Hour))

	otherKey, err := jwt.New("other-secret", time.Hour).GenerateToken("user-id-123", nil)
	require.NoError(t, err)

	tests := []struct {
		name          string
		authorization string
		detail        string
	}{
		{"missing header", "", "missing authorization header"},
		{"invalid format", "Basic xxx", "invalid authorization header format"},
		{"bare token", otherKey, "invalid authorization header format"},
		{"invalid token", "Bearer invalid", "invalid or expired token"},
		{"another key's token", "Bearer " + otherKey, "invalid or expired token"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp := get(t, handler, "/private", tc.authorization)

			assert.Equal(t, http.StatusUnauthorized, resp.Code)
			assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))
			assert.JSONEq(t,
				`{"type":"about:blank","title":"Unauthorized","status":401,"detail":"`+tc.detail+`"}`,
				resp.Body.String())
		})
	}
}

func TestAuthRunsTheOperationWithTheTokensCaller(t *testing.T) {
	t.Parallel()

	tokens := jwt.New("test-secret", time.Hour)
	before := time.Now().Truncate(time.Second)

	token, err := tokens.GenerateToken("user-id-123", []string{"admin", "superuser"})
	require.NoError(t, err)

	resp := get(t, newTestHandler(tokens), "/private", "Bearer "+token)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var got callerOutput

	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &got.Body))
	assert.True(t, got.Body.HasCaller)
	assert.Equal(t, "user-id-123", got.Body.UserID)
	assert.Equal(t, []entity.Role{entity.RoleAdmin}, got.Body.Roles, "unknown roles are dropped")
	assert.False(t, got.Body.IssuedAt.Before(before), "the token's issue time is kept: %s", got.Body.IssuedAt)
}

// An operation that declares no Security is public: Auth does not read a token for it, valid or not.
func TestAuthLetsAPublicOperationThrough(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(jwt.New("test-secret", time.Hour))

	for _, authorization := range []string{"", "Bearer invalid"} {
		resp := get(t, handler, "/public", authorization)
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

		var got callerOutput

		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &got.Body))
		assert.False(t, got.Body.HasCaller, authorization)
	}
}
