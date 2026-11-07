package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// callerOf sends a request with the given Authorization header through WithOptionalCaller, and
// returns what Caller answered in the handler behind it.
func callerOf(t *testing.T, manager *jwt.Manager, header string) (authn.Caller, error) {
	t.Helper()

	var (
		got authn.Caller
		err error
	)

	h := middleware.WithOptionalCaller(manager, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, err = middleware.Caller(r.Context())
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", http.NoBody)
	if header != "" {
		req.Header.Set("Authorization", header)
	}

	h.ServeHTTP(httptest.NewRecorder(), req)

	return got, err
}

func TestWithOptionalCallerReadsTheToken(t *testing.T) {
	t.Parallel()

	manager := jwt.New("secret", time.Hour)

	token, err := manager.GenerateToken("user-1", []string{"admin"})
	require.NoError(t, err)

	got, err := callerOf(t, manager, "Bearer "+token)
	require.NoError(t, err)

	assert.Equal(t, entity.Principal{UserID: "user-1", Roles: []entity.Role{entity.RoleAdmin}}, got.Principal)
}

// A request without a valid bearer token gets through without a caller, and a resolver that asks
// for one gets UNAUTHENTICATED, the code the web client signs out on.
func TestWithOptionalCallerAnonymous(t *testing.T) {
	t.Parallel()

	manager := jwt.New("secret", time.Hour)

	token, err := manager.GenerateToken("user-1", nil)
	require.NoError(t, err)

	for name, header := range map[string]string{
		"no header":     "",
		"not a bearer":  "Basic " + token,
		"invalid token": "Bearer invalid",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := callerOf(t, manager, header)

			var gqlErr *gqlerror.Error

			require.ErrorAs(t, err, &gqlErr)
			assert.Equal(t, "UNAUTHENTICATED", gqlErr.Extensions["code"])
		})
	}
}
