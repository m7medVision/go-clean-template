package restapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	resttest.Main(m)
}

// The router-wide tests reach only these use-case methods; each v1 domain folder fakes its own use
// case in full.

type fakeUser struct {
	usecase.User

	registered entity.User
}

func (f *fakeUser) GetUser(context.Context, string) (entity.User, error) {
	return f.registered, nil
}

type fakeTask struct {
	usecase.Task

	stored entity.Task
}

func (f *fakeTask) Update(context.Context, string, string, string, string) (entity.Task, error) {
	return f.stored, nil
}

type fakeExchange struct {
	usecase.Exchange
}

type panickingTask struct {
	usecase.Task
}

func (panickingTask) Get(context.Context, string, string) (entity.Task, error) {
	panic("boom")
}

func TestOptionalFeaturesServeNothingWithoutTheirOption(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(resttest.Config(t), &fakeExchange{}, &fakeUser{}, &fakeTask{})

	for _, tt := range []struct{ method, target string }{
		{http.MethodGet, "/v1/auth/oidc"},
		{http.MethodPost, "/v1/auth/oidc/begin"},
		{http.MethodPost, "/v1/auth/passkey/begin"},
		{http.MethodGet, "/v1/passkeys"},
	} {
		resp := resttest.Do(t.Context(), handler, tt.method, tt.target, resttest.Token(t), "{}")
		assert.Equal(t, http.StatusNotFound, resp.Code, tt.target)
	}

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/user/profile", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code, "core routes are still served")
}

func TestApplianceRoutesArePublic(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(resttest.Config(t), &fakeExchange{}, &fakeUser{}, &fakeTask{})

	for _, target := range []string{"/healthz", "/metrics", "/openapi.json", "/docs"} {
		resp := resttest.Do(t.Context(), handler, http.MethodGet, target, "", "")
		assert.Equal(t, http.StatusOK, resp.Code, target)
	}
}

func TestUnauthenticatedRequestsAreRejected(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(resttest.Config(t), &fakeExchange{}, &fakeUser{}, &fakeTask{})

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/tasks", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))
}

func TestResponseBodyHasNoSchemaLink(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(resttest.Config(t), &fakeExchange{}, &fakeUser{
		registered: entity.User{ID: "user-1"},
	}, &fakeTask{})

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/user/profile", resttest.Token(t), "")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.NotContains(t, resp.Body.String(), "$schema")
	assert.Empty(t, resp.Header().Get("Link"))
}

func TestUnknownBodyFieldsAreIgnored(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(resttest.Config(t), &fakeExchange{}, &fakeUser{}, &fakeTask{
		stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo},
	})

	resp := resttest.Do(t.Context(), handler, http.MethodPut, "/v1/tasks/task-1", resttest.Token(t),
		`{"id":"task-1","title":"t","description":"","status":"todo","created_at":"2026-01-01T00:00:00Z"}`)
	assert.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
}

func TestValidationErrorDoesNotEchoInput(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(resttest.Config(t), &fakeExchange{}, &fakeUser{}, &fakeTask{})

	resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/auth/register", "",
		`{"username":"johndoe","email":"john@example.com","password":"s3cr"}`)
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), "body.password")
	assert.NotContains(t, resp.Body.String(), "s3cr")
}

func TestAuthExemptionIsLimitedToAuthRoutes(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(resttest.Config(t), &fakeExchange{}, &fakeUser{}, &fakeTask{})

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/authors", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestPanicAnswers500AndIsCounted(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(resttest.Config(t), &fakeExchange{}, &fakeUser{}, panickingTask{})

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/tasks/task-1", resttest.Token(t), "")
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))

	metrics := resttest.Do(t.Context(), handler, http.MethodGet, "/metrics", "", "")
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
		"/v1/auth/passkey/begin", "/v1/auth/passkey/finish",
		"/v1/auth/oidc", "/v1/auth/oidc/begin", "/v1/auth/oidc/finish",
	} {
		assert.Contains(t, spec.Paths, path)
	}
}
