package restapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/task"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/user"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	userquery "github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The handler-wide tests reach only these use-case methods, through the user and task operations;
// each v1 domain folder fakes its own use case in full.

func newRouter(b cqrs.Buses) http.Handler {
	return resttest.NewRouter(b, user.NewRoutes, task.NewRoutes)
}

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

func TestDocsArePublic(t *testing.T) {
	t.Parallel()

	handler := newRouter(taskBuses(&entity.Task{}))

	for _, target := range []string{"/openapi.json", "/docs"} {
		resp := resttest.Do(t.Context(), handler, http.MethodGet, target, "", "")
		assert.Equal(t, http.StatusOK, resp.Code, target)
	}
}

func TestUnauthenticatedRequestsAreRejected(t *testing.T) {
	t.Parallel()

	handler := newRouter(taskBuses(&entity.Task{}))

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/tasks", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Equal(t, "application/problem+json", resp.Header().Get("Content-Type"))
}

func TestResponseBodyHasNoSchemaLink(t *testing.T) {
	t.Parallel()

	handler := newRouter(userBuses(&entity.User{ID: "user-1"}))

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/user/profile", resttest.Token(t), "")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.NotContains(t, resp.Body.String(), "$schema")
	assert.Empty(t, resp.Header().Get("Link"))
}

func TestUnknownBodyFieldsAreIgnored(t *testing.T) {
	t.Parallel()

	handler := newRouter(taskBuses(&entity.Task{ID: "task-1", Status: entity.TaskStatusTodo}))

	resp := resttest.Do(t.Context(), handler, http.MethodPut, "/v1/tasks/task-1", resttest.Token(t),
		`{"id":"task-1","title":"t","description":"","status":"todo","created_at":"2026-01-01T00:00:00Z"}`)
	assert.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
}

func TestValidationErrorDoesNotEchoInput(t *testing.T) {
	t.Parallel()

	handler := newRouter(taskBuses(&entity.Task{}))

	resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/auth/register", "",
		`{"username":"johndoe","email":"john@example.com","password":"s3cr"}`)
	require.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), "body.password")
	assert.NotContains(t, resp.Body.String(), "s3cr")
}

func TestAuthExemptionIsLimitedToAuthRoutes(t *testing.T) {
	t.Parallel()

	handler := newRouter(taskBuses(&entity.Task{}))

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/authors", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}
