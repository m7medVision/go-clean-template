package restapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/shopspring/decimal"
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

type fakeExchange struct {
	usecase.Exchange

	converted entity.Conversion
	err       error

	// gotLimit and gotOffset record what History was called with.
	gotLimit, gotOffset int
}

func (f *fakeExchange) Convert(context.Context, string, string, string, decimal.Decimal) (entity.Conversion, error) {
	return f.converted, f.err
}

func (f *fakeExchange) History(_ context.Context, _ string, limit, offset int) (entity.ExchangeHistory, error) {
	f.gotLimit, f.gotOffset = limit, offset

	return entity.ExchangeHistory{History: []entity.Conversion{f.converted}, Total: 11}, f.err
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

func newRouter(cfg *config.Config, e usecase.Exchange, u usecase.User, tk usecase.Task) http.Handler {
	return newRouterWithPasskey(cfg, e, u, tk, &fakePasskey{})
}

func newRouterWithPasskey(
	cfg *config.Config,
	e usecase.Exchange,
	u usecase.User,
	tk usecase.Task,
	pk usecase.Passkey,
) http.Handler {
	jwtManager := jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry)

	return restapi.NewRouter(cfg, e, u, tk, pk, jwtManager, noopLogger{})
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

	handler := newRouter(newTestConfig(t), &fakeExchange{}, &fakeUser{}, &fakeTask{})

	for _, target := range []string{"/healthz", "/metrics", "/openapi.json", "/docs"} {
		resp := doRequest(t.Context(), handler, http.MethodGet, target, "", "")
		assert.Equal(t, http.StatusOK, resp.Code, target)
	}
}

func TestUnauthenticatedRequestsAreRejected(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeExchange{}, &fakeUser{}, &fakeTask{})

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

		handler := newRouter(cfg, &fakeExchange{}, &fakeUser{}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/tasks", token, `{"title":""}`)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	t.Run("invalid json body is rejected", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(cfg, &fakeExchange{}, &fakeUser{}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/tasks", token, "{invalid json}")
		assert.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(cfg, &fakeExchange{}, &fakeUser{}, &fakeTask{
			stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo},
		})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/tasks", token, `{"title":"my task"}`)
		assert.Equal(t, http.StatusCreated, resp.Code)
	})

	t.Run("trailing slash is stripped", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(cfg, &fakeExchange{}, &fakeUser{}, &fakeTask{
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
		handler := newRouter(cfg, &fakeExchange{}, &fakeUser{
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
		handler := newRouter(cfg, &fakeExchange{}, &fakeUser{
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
		handler := newRouter(cfg, &fakeExchange{}, &fakeUser{}, &fakeTask{err: entity.ErrTaskNotFound})

		resp := doRequest(t.Context(), handler, http.MethodDelete, "/v1/tasks/task-1", token, "")
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("success maps to 204", func(t *testing.T) {
		t.Parallel()

		cfg := newTestConfig(t)
		handler := newRouter(cfg, &fakeExchange{}, &fakeUser{}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodDelete, "/v1/tasks/task-1", token, "")
		assert.Equal(t, http.StatusNoContent, resp.Code)
	})
}

func TestTaskForbiddenMapsTo403(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(t)
	handler := newRouter(cfg, &fakeExchange{}, &fakeUser{}, &fakeTask{err: entity.ErrTaskForbidden})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/tasks/task-1", newToken(t), "")
	assert.Equal(t, http.StatusForbidden, resp.Code)
}

func TestInvalidTransitionMapsTo400(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(t)
	handler := newRouter(cfg, &fakeExchange{}, &fakeUser{}, &fakeTask{err: entity.ErrInvalidTransition})

	resp := doRequest(t.Context(), handler, http.MethodPatch, "/v1/tasks/task-1/status",
		newToken(t), `{"status":"done"}`)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestUsecaseFailureMapsTo500(t *testing.T) {
	t.Parallel()

	cfg := newTestConfig(t)
	handler := newRouter(cfg, &fakeExchange{}, &fakeUser{}, &fakeTask{err: io.EOF})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/tasks", newToken(t), "")
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestConvertExchange(t *testing.T) {
	t.Parallel()

	token := newToken(t)
	body := `{"from":"eur","to":"USD","amount":"100.10"}`
	converted := entity.Conversion{
		From: "EUR", To: "USD",
		Amount:   decimal.RequireFromString("100.10"),
		Rate:     decimal.RequireFromString("1.1225"),
		Result:   decimal.RequireFromString("112.36"),
		RateDate: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
	}

	t.Run("success returns amounts as strings and the rate date", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(newTestConfig(t), &fakeExchange{converted: converted}, &fakeUser{}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/exchange/convert", token, body)
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		assert.JSONEq(t,
			`{"from":"EUR","to":"USD","amount":"100.1","rate":"1.1225","result":"112.36","rate_date":"2026-10-19"}`,
			resp.Body.String())
	})

	t.Run("malformed amount is rejected by validation", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(newTestConfig(t), &fakeExchange{converted: converted}, &fakeUser{}, &fakeTask{})

		resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/exchange/convert", token,
			`{"from":"EUR","to":"USD","amount":"1e3"}`)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})

	for _, tt := range []struct {
		name string
		err  error
		code int
	}{
		{"invalid conversion maps to 400", entity.ErrInvalidConversion, http.StatusBadRequest},
		{"unsupported currency maps to 400", entity.ErrUnsupportedCurrency, http.StatusBadRequest},
		{"other failure maps to 500", io.EOF, http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := newRouter(newTestConfig(t), &fakeExchange{err: tt.err}, &fakeUser{}, &fakeTask{})

			resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/exchange/convert", token, body)
			assert.Equal(t, tt.code, resp.Code)
		})
	}
}

func TestExchangeHistory(t *testing.T) {
	t.Parallel()

	exchange := &fakeExchange{converted: entity.Conversion{
		From: "EUR", To: "USD", Amount: decimal.RequireFromString("1"),
		Rate: decimal.RequireFromString("1.1225"), Result: decimal.RequireFromString("1.12"),
		RateDate: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
	}}
	handler := newRouter(newTestConfig(t), exchange, &fakeUser{}, &fakeTask{})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/exchange/history?limit=5&offset=10", newToken(t), "")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.JSONEq(t,
		`{"history":[{"from":"EUR","to":"USD","amount":"1","rate":"1.1225","result":"1.12","rate_date":"2026-10-19"}],"total":11}`,
		resp.Body.String())
	assert.Equal(t, 5, exchange.gotLimit)
	assert.Equal(t, 10, exchange.gotOffset)
}

type panickingTask struct {
	usecase.Task
}

func (panickingTask) Get(context.Context, string, string) (entity.Task, error) {
	panic("boom")
}

func TestResponseBodyHasNoSchemaLink(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeExchange{}, &fakeUser{
		registered: entity.User{ID: "user-1"},
	}, &fakeTask{})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/user/profile", newToken(t), "")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.NotContains(t, resp.Body.String(), "$schema")
	assert.Empty(t, resp.Header().Get("Link"))
}

func TestUnknownBodyFieldsAreIgnored(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeExchange{}, &fakeUser{}, &fakeTask{
		stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo},
	})

	resp := doRequest(t.Context(), handler, http.MethodPut, "/v1/tasks/task-1", newToken(t),
		`{"id":"task-1","title":"t","description":"","status":"todo","created_at":"2026-01-01T00:00:00Z"}`)
	assert.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
}

func TestValidationErrorDoesNotEchoInput(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeExchange{}, &fakeUser{}, &fakeTask{})

	resp := doRequest(t.Context(), handler, http.MethodPost, "/v1/auth/register", "",
		`{"username":"johndoe","email":"john@example.com","password":"s3cr"}`)
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), "body.password")
	assert.NotContains(t, resp.Body.String(), "s3cr")
}

func TestAuthExemptionIsLimitedToAuthRoutes(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeExchange{}, &fakeUser{}, &fakeTask{})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/authors", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestPanicAnswers500AndIsCounted(t *testing.T) {
	t.Parallel()

	handler := newRouter(newTestConfig(t), &fakeExchange{}, &fakeUser{}, panickingTask{})

	resp := doRequest(t.Context(), handler, http.MethodGet, "/v1/tasks/task-1", newToken(t), "")
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))

	metrics := doRequest(t.Context(), handler, http.MethodGet, "/metrics", "", "")
	body, err := io.ReadAll(metrics.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `http_requests_total{code="500",method="GET",route="/v1/tasks/{id}"}`)
}

func TestOpenAPIExportsEveryOperation(t *testing.T) {
	t.Parallel()

	doc, err := restapi.OpenAPI("test")
	require.NoError(t, err)

	var spec struct {
		Info  struct{ Version string } `json:"info"`
		Paths map[string]any           `json:"paths"`
	}

	require.NoError(t, json.Unmarshal(doc, &spec))
	assert.Equal(t, "test", spec.Info.Version)

	for _, path := range []string{
		"/v1/auth/register", "/v1/auth/login", "/v1/user/profile",
		"/v1/tasks", "/v1/tasks/{id}", "/v1/tasks/{id}/status",
		"/v1/exchange/convert", "/v1/exchange/history",
		"/v1/passkeys", "/v1/passkeys/register/begin", "/v1/passkeys/register/finish",
	} {
		assert.Contains(t, spec.Paths, path)
	}
}
