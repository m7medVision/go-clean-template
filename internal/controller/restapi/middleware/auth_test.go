package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noopLogger struct{}

func (noopLogger) Debug(any, ...any)   {}
func (noopLogger) Info(string, ...any) {}
func (noopLogger) Warn(string, ...any) {}
func (noopLogger) Error(any, ...any)   {}
func (noopLogger) Fatal(any, ...any)   {}

var _ logger.Interface = noopLogger{}

func newTestHandler(jwtManager *jwt.Manager) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Auth(jwtManager, noopLogger{}))
	r.Get("/v1/test", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserID(r.Context())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		fmt.Fprint(w, userID)
	})

	return r
}

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	jwtManager := jwt.New("test-secret", time.Hour)
	handler := newTestHandler(jwtManager)

	validToken, err := jwtManager.GenerateToken("user-id-123")
	require.NoError(t, err)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "missing header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid format",
			authHeader:     "Basic xxx",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "valid token",
			authHeader:     "Bearer " + validToken,
			expectedStatus: http.StatusOK,
			expectedBody:   "user-id-123",
		},
	}

	for _, tc := range tests {
		localTc := tc

		t.Run(localTc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/test", http.NoBody)
			if localTc.authHeader != "" {
				req.Header.Set("Authorization", localTc.authHeader)
			}

			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)

			assert.Equal(t, localTc.expectedStatus, resp.Code)

			if localTc.expectedBody != "" {
				assert.Equal(t, localTc.expectedBody, resp.Body.String())
			}
		})
	}
}

func TestAuthMiddlewareAuthTime(t *testing.T) {
	t.Parallel()

	jwtManager := jwt.New("test-secret", time.Hour)

	var (
		authTime time.Time
		ok       bool
	)

	r := chi.NewRouter()
	r.Use(middleware.Auth(jwtManager, noopLogger{}))
	r.Get("/v1/test", func(_ http.ResponseWriter, r *http.Request) {
		authTime, ok = middleware.AuthTime(r.Context())
	})

	before := time.Now().Truncate(time.Second)

	token, err := jwtManager.GenerateToken("user-id-123")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)

	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.True(t, ok)
	assert.False(t, authTime.Before(before), "auth time %s, before %s", authTime, before)
	assert.False(t, authTime.After(time.Now()), "auth time in the future: %s", authTime)
}

func TestAuthTimeWithoutAuth(t *testing.T) {
	t.Parallel()

	_, ok := middleware.AuthTime(t.Context())
	assert.False(t, ok)
}
