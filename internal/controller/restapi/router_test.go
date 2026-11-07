package restapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	userquery "github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The router-wide tests reach only these use-case methods; each v1 domain folder fakes its own use
// case in full.

// userBuses read registered back with GetUser.
func userBuses(registered *entity.User) cqrs.Buses {
	b := resttest.NewBuses()
	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[userquery.GetUser, entity.User](
		func(context.Context, userquery.GetUser) (entity.User, error) { return *registered, nil }))

	return b
}

// taskBuses answers UpdateTask, and reads stored back with GetTask.
func taskBuses(stored *entity.Task) cqrs.Buses {
	cmds, qs := cqrs.NewCommands(), cqrs.NewQueries()

	cqrs.HandleCommand(cmds, cqrs.HandlerFunc[command.UpdateTask, entity.Written](
		func(context.Context, command.UpdateTask) (entity.Written, error) {
			return entity.Written{ID: stored.ID}, nil
		}))
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.GetTask, entity.Task](
		func(context.Context, query.GetTask) (entity.Task, error) { return *stored, nil }))

	return cqrs.Buses{Commands: cmds, Queries: qs}
}

// panickingBuses panic on GetTask.
func panickingBuses() cqrs.Buses {
	qs := cqrs.NewQueries()
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.GetTask, entity.Task](
		func(context.Context, query.GetTask) (entity.Task, error) { panic("boom") }))

	return cqrs.Buses{Queries: qs}
}

func TestOptionalFeaturesServeNothingWithoutTheirOption(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(taskBuses(&entity.Task{}))

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

	handler := resttest.NewRouter(taskBuses(&entity.Task{}))

	for _, target := range []string{"/healthz", "/metrics", "/openapi.json", "/docs"} {
		resp := resttest.Do(t.Context(), handler, http.MethodGet, target, "", "")
		assert.Equal(t, http.StatusOK, resp.Code, target)
	}
}

func TestUnauthenticatedRequestsAreRejected(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(taskBuses(&entity.Task{}))

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/tasks", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))
}

func TestResponseBodyHasNoSchemaLink(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(userBuses(&entity.User{ID: "user-1"}))

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/user/profile", resttest.Token(t), "")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.NotContains(t, resp.Body.String(), "$schema")
	assert.Empty(t, resp.Header().Get("Link"))
}

func TestUnknownBodyFieldsAreIgnored(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(taskBuses(&entity.Task{ID: "task-1", Status: entity.TaskStatusTodo}))

	resp := resttest.Do(t.Context(), handler, http.MethodPut, "/v1/tasks/task-1", resttest.Token(t),
		`{"id":"task-1","title":"t","description":"","status":"todo","created_at":"2026-01-01T00:00:00Z"}`)
	assert.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
}

func TestValidationErrorDoesNotEchoInput(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(taskBuses(&entity.Task{}))

	resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/auth/register", "",
		`{"username":"johndoe","email":"john@example.com","password":"s3cr"}`)
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), "body.password")
	assert.NotContains(t, resp.Body.String(), "s3cr")
}

func TestAuthExemptionIsLimitedToAuthRoutes(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(taskBuses(&entity.Task{}))

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/authors", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestPanicAnswers500AndIsCounted(t *testing.T) {
	t.Parallel()

	handler := resttest.NewRouter(panickingBuses())

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

// The schema hints of every list's limit document the domain's paging rule (entity.Page).
func TestEveryLimitDocumentsTheDomainsPage(t *testing.T) {
	t.Parallel()

	doc, err := restapi.OpenAPI("test")
	require.NoError(t, err)

	var spec struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name   string `json:"name"`
				Schema struct {
					Default float64 `json:"default"`
					Maximum float64 `json:"maximum"`
				} `json:"schema"`
			} `json:"parameters"`
		} `json:"paths"`
	}

	require.NoError(t, json.Unmarshal(doc, &spec))

	limits := 0

	for path, ops := range spec.Paths {
		for method, op := range ops {
			for _, p := range op.Parameters {
				if p.Name != "limit" {
					continue
				}

				limits++

				assert.InDelta(t, entity.DefaultPageLimit, p.Schema.Default, 0, method+" "+path)
				assert.InDelta(t, entity.MaxPageLimit, p.Schema.Maximum, 0, method+" "+path)
			}
		}
	}

	assert.Positive(t, limits)
}

// The REST schema's length and pattern hints document the domain's rules (entity.ValidateTaskText,
// entity.ValidateSignUp, entity.NewConversion, the passkey name), which every transport enforces.
func TestBodyLengthHintsDocumentTheDomainsRules(t *testing.T) {
	t.Parallel()

	doc, err := restapi.OpenAPI("test")
	require.NoError(t, err)

	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					MinLength *int   `json:"minLength"`
					MaxLength *int   `json:"maxLength"`
					Pattern   string `json:"pattern"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}

	require.NoError(t, json.Unmarshal(doc, &spec))

	schemas := spec.Components.Schemas

	for _, name := range []string{"CreateTask", "UpdateTask"} {
		assert.Equal(t, 1, *schemas[name].Properties["title"].MinLength, name)
		assert.Equal(t, entity.MaxTaskTitleLen, *schemas[name].Properties["title"].MaxLength, name)
		assert.Equal(t, entity.MaxTaskDescriptionLen, *schemas[name].Properties["description"].MaxLength, name)
	}

	assert.Equal(t, entity.MinUsernameLen, *schemas["Register"].Properties["username"].MinLength)
	assert.Equal(t, entity.MaxUsernameLen, *schemas["Register"].Properties["username"].MaxLength)
	assert.Equal(t, entity.MinPasswordLen, *schemas["Register"].Properties["password"].MinLength)
	assert.Equal(t, entity.MaxPasskeyNameLen, *schemas["RenamePasskey"].Properties["name"].MaxLength)
	assert.Equal(t, fmt.Sprintf(`^[0-9]{1,%d}(\.[0-9]{1,%d})?$`, entity.MaxAmountDigits, entity.MaxAmountScale),
		schemas["Convert"].Properties["amount"].Pattern)
}
