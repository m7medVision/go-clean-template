package restapi_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/config"
	"github.com/evrone/go-clean-template/internal/controller/restapi"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noopLogger struct{}

var _ logger.Interface = noopLogger{}

func (noopLogger) Debug(any, ...any)   {}
func (noopLogger) Info(string, ...any) {}
func (noopLogger) Warn(string, ...any) {}
func (noopLogger) Error(any, ...any)   {}
func (noopLogger) Fatal(any, ...any)   {}

type fakeUser struct {
	usecase.User

	registered  entity.User
	registerErr error
	token       string
	loginErr    error
	getErr      error
}

func (f *fakeUser) Register(context.Context, string, string, string) (entity.User, error) {
	return f.registered, f.registerErr
}

func (f *fakeUser) Login(context.Context, string, string) (string, error) {
	return f.token, f.loginErr
}

func (f *fakeUser) GetUser(context.Context, string) (entity.User, error) {
	return f.registered, f.getErr
}

type fakeTask struct {
	usecase.Task

	stored entity.Task
	err    error
}

func (f *fakeTask) Create(context.Context, string, string, string) (entity.Task, error) {
	return f.stored, f.err
}

func (f *fakeTask) Get(context.Context, string, string) (entity.Task, error) {
	return f.stored, f.err
}

func (f *fakeTask) List(context.Context, string, *entity.TaskStatus, int, int) ([]entity.Task, int, error) {
	return []entity.Task{f.stored}, 1, f.err
}

func (f *fakeTask) Update(context.Context, string, string, string, string) (entity.Task, error) {
	return f.stored, f.err
}

func (f *fakeTask) Transition(context.Context, string, string, entity.TaskStatus) (entity.Task, error) {
	return f.stored, f.err
}

func (f *fakeTask) Delete(context.Context, string, string) error {
	return f.err
}

type fakeTranslation struct {
	usecase.Translation

	translated entity.Translation
	err        error
}

func (f *fakeTranslation) Translate(_ context.Context, _ string, _ entity.Translation) (entity.Translation, error) {
	return f.translated, f.err
}

func (f *fakeTranslation) History(context.Context, string) (entity.TranslationHistory, error) {
	return entity.TranslationHistory{}, f.err
}

func TestMain(m *testing.M) {
	for _, env := range []struct{ key, value string }{
		{"APP_NAME", "test"},
		{"APP_VERSION", "1.0.0"},
		{"HTTP_PORT", "0"},
		{"LOG_LEVEL", "info"},
		{"PG_POOL_MAX", "1"},
		{"PG_URL", "postgres://localhost/db"},
		{"GRPC_PORT", "0"},
		{"JWT_SECRET", "test-secret"},
		{"DOCS_ENABLED", "true"},
	} {
		if err := os.Setenv(env.key, env.value); err != nil {
			panic(err)
		}
	}

	os.Exit(m.Run())
}

func newTestConfig(t *testing.T) *config.Config {
	t.Helper()

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	return cfg
}

func newRouter(cfg *config.Config, t usecase.Translation, u usecase.User, tk usecase.Task) http.Handler {
	jwtManager := jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry)

	return restapi.NewRouter(cfg, t, u, tk, jwtManager, noopLogger{})
}

func newToken(t *testing.T) string {
	t.Helper()

	token, err := jwt.New("test-secret", time.Hour).GenerateToken("user-1")
	require.NoError(t, err)

	return token
}

func doRequest(
	ctx context.Context,
	handler http.Handler,
	method, target, token, body string,
) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	return resp
}

func TestApplianceRoutesArePublic(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeTranslation{}, &fakeUser{}, &fakeTask{})

	for _, target := range []string{"/healthz", "/metrics", "/openapi.json", "/docs"} {
		resp := doRequest(t.Context(), handler, http.MethodGet, target, "", "")
		assert.Equal(t, http.StatusOK, resp.Code, target)
	}
}

func TestUnauthenticatedRequestsAreRejected(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeTranslation{}, &fakeUser{}, &fakeTask{})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/tasks", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))
}

func TestCreateTask(t *testing.T) {
	t.Parallel()

	token := newToken(t)
	cfg := newTestConfig(t)

	t.Run("empty title is rejected", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(cfg, &fakeTranslation{}, &fakeUser{}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/tasks", token, `{"title":""}`)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("invalid json body is rejected", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(cfg, &fakeTranslation{}, &fakeUser{}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/tasks", token, "{invalid json}")
		assert.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(cfg, &fakeTranslation{}, &fakeUser{}, &fakeTask{
			stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo},
		})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/tasks", token, `{"title":"my task"}`)
		assert.Equal(t, http.StatusCreated, resp.Code)
	})

	t.Run("trailing slash is stripped", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(cfg, &fakeTranslation{}, &fakeUser{}, &fakeTask{
			stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo},
		})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/tasks/", token, `{"title":"my task"}`)
		assert.Equal(t, http.StatusCreated, resp.Code)
	})
}

func TestRegisterAndLogin(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		cfg := newTestConfig(t)
		handler := newRouter(cfg, &fakeTranslation{}, &fakeUser{
			registered: entity.User{ID: "user-1", Username: "johndoe"},
			token:      "token-1",
		}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/auth/register", "",
			`{"username":"johndoe","email":"john@example.com","password":"secret123"}`)
		assert.Equal(t, http.StatusCreated, resp.Code)

		resp = doRequest(t.Context(), handler, http.MethodPost, "/v1/auth/login", "",
			`{"email":"john@example.com","password":"secret123"}`)
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("conflict and bad credentials", func(t *testing.T) {
		t.Parallel()

		cfg := newTestConfig(t)
		handler := newRouter(cfg, &fakeTranslation{}, &fakeUser{
			registerErr: entity.ErrUserAlreadyExists,
			loginErr:    entity.ErrInvalidCredentials,
		}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/auth/register", "",
			`{"username":"johndoe","email":"john@example.com","password":"secret123"}`)
		assert.Equal(t, http.StatusConflict, resp.Code)

		resp = doRequest(t.Context(), handler, http.MethodPost, "/v1/auth/login", "",
			`{"email":"john@example.com","password":"secret123"}`)
		assert.Equal(t, http.StatusUnauthorized, resp.Code)
	})
}

func TestDeleteTask(t *testing.T) {
	t.Parallel()

	token := newToken(t)

	t.Run("not found maps to 404", func(t *testing.T) {
		t.Parallel()

		cfg := newTestConfig(t)
		handler := newRouter(cfg, &fakeTranslation{}, &fakeUser{}, &fakeTask{err: entity.ErrTaskNotFound})

		resp := doRequest(t.Context(), handler, http.MethodDelete, "/v1/tasks/task-1", token, "")
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("success maps to 204", func(t *testing.T) {
		t.Parallel()

		cfg := newTestConfig(t)
		handler := newRouter(cfg, &fakeTranslation{}, &fakeUser{}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodDelete, "/v1/tasks/task-1", token, "")
		assert.Equal(t, http.StatusNoContent, resp.Code)
	})
}

func TestTaskForbiddenMapsTo403(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(t)
	handler := newRouter(cfg, &fakeTranslation{}, &fakeUser{}, &fakeTask{err: entity.ErrTaskForbidden})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/tasks/task-1", newToken(t), "")
	assert.Equal(t, http.StatusForbidden, resp.Code)
}

func TestInvalidTransitionMapsTo400(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(t)
	handler := newRouter(cfg, &fakeTranslation{}, &fakeUser{}, &fakeTask{err: entity.ErrInvalidTransition})

	resp := doRequest(t.Context(), handler, http.MethodPatch, "/v1/tasks/task-1/status",
		newToken(t), `{"status":"done"}`)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestUsecaseFailureMapsTo500(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(t)
	handler := newRouter(cfg, &fakeTranslation{}, &fakeUser{}, &fakeTask{err: io.EOF})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/tasks", newToken(t), "")
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestDoTranslate(t *testing.T) {
	t.Parallel()

	token := newToken(t)
	body := `{"source":"auto","destination":"en","original":"текст"}`

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		cfg := newTestConfig(t)
		handler := newRouter(cfg, &fakeTranslation{translated: entity.Translation{Translation: "text"}},
			&fakeUser{}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/translation/do-translate", token, body)
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("failure maps to 500", func(t *testing.T) {
		t.Parallel()

		cfg := newTestConfig(t)
		handler := newRouter(cfg, &fakeTranslation{err: io.EOF}, &fakeUser{}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/translation/do-translate", token, body)
		assert.Equal(t, http.StatusInternalServerError, resp.Code)
	})
}

type panickingTask struct {
	usecase.Task
}

func (panickingTask) Get(context.Context, string, string) (entity.Task, error) {
	panic("boom")
}

func TestResponseBodyHasNoSchemaLink(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeTranslation{}, &fakeUser{
		registered: entity.User{ID: "user-1"},
	}, &fakeTask{})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/user/profile", newToken(t), "")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.NotContains(t, resp.Body.String(), "$schema")
	assert.Empty(t, resp.Header().Get("Link"))
}

func TestUnknownBodyFieldsAreIgnored(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeTranslation{}, &fakeUser{}, &fakeTask{
		stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo},
	})

	resp := doRequest(t.Context(), handler, http.MethodPut, "/v1/tasks/task-1", newToken(t),
		`{"id":"task-1","title":"t","description":"","status":"todo","created_at":"2026-01-01T00:00:00Z"}`)
	assert.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
}

func TestValidationErrorDoesNotEchoInput(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeTranslation{}, &fakeUser{}, &fakeTask{})

	resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/auth/register", "",
		`{"username":"johndoe","email":"john@example.com","password":"s3cr"}`)
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), "body.password")
	assert.NotContains(t, resp.Body.String(), "s3cr")
}

func TestAuthExemptionIsLimitedToAuthRoutes(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeTranslation{}, &fakeUser{}, &fakeTask{})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/authors", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestPanicAnswers500AndIsCounted(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeTranslation{}, &fakeUser{}, panickingTask{})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/tasks/task-1", newToken(t), "")
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))

	metrics := doRequest(t.Context(), handler, http.MethodGet, "/metrics", "", "")
	body, err := io.ReadAll(metrics.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `http_requests_total{code="500",method="GET",route="/v1/tasks/{id}"}`)
}
