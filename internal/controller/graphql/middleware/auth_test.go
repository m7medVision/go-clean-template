package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithOptionalCallerPrincipal(t *testing.T) {
	t.Parallel()

	manager := jwt.New("secret", time.Hour)

	token, err := manager.GenerateToken("user-1", []string{"user", "admin", "root"})
	require.NoError(t, err)

	var (
		got entity.Principal
		ok  bool
	)

	h := middleware.WithOptionalCaller(manager, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, ok = middleware.Principal(r.Context())
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, ok)
	assert.Equal(t, entity.Principal{UserID: "user-1", Roles: []entity.Role{entity.RoleUser, entity.RoleAdmin}}, got)
}

func TestWithOptionalCallerAnonymous(t *testing.T) {
	t.Parallel()

	var ok bool

	h := middleware.WithOptionalCaller(jwt.New("secret", time.Hour), http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, ok = middleware.Principal(r.Context())
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", http.NoBody))

	assert.False(t, ok)
}
