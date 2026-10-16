package graphql_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/graphql"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _userID = "user-1"

type noopLogger struct{}

var _ logger.Interface = noopLogger{}

func (noopLogger) Debug(any, ...any)   {}
func (noopLogger) Info(string, ...any) {}
func (noopLogger) Warn(string, ...any) {}
func (noopLogger) Error(any, ...any)   {}
func (noopLogger) Fatal(any, ...any)   {}

type fakeTask struct {
	usecase.Task

	stored     entity.Task
	err        error
	panicMsg   string
	called     bool
	listStatus *entity.TaskStatus
}

func (f *fakeTask) call() {
	f.called = true

	if f.panicMsg != "" {
		panic(f.panicMsg)
	}
}

func (f *fakeTask) Create(_ context.Context, userID, title, description string) (entity.Task, error) {
	f.call()

	return entity.Task{
		ID: "task-1", UserID: userID, Title: title, Description: description, Status: entity.TaskStatusTodo,
	}, f.err
}

func (f *fakeTask) Get(context.Context, string, string) (entity.Task, error) {
	f.call()

	return f.stored, f.err
}

func (f *fakeTask) List(_ context.Context, _ string, status *entity.TaskStatus, _, _ int) ([]entity.Task, int, error) {
	f.call()
	f.listStatus = status

	return []entity.Task{f.stored}, 1, f.err
}

func (f *fakeTask) Update(_ context.Context, _, _, title, description string) (entity.Task, error) {
	f.call()

	updated := f.stored
	updated.Title = title
	updated.Description = description

	return updated, f.err
}

func (f *fakeTask) Transition(_ context.Context, _, _ string, status entity.TaskStatus) (entity.Task, error) {
	f.call()

	moved := f.stored
	moved.Status = status

	return moved, f.err
}

func (f *fakeTask) Delete(context.Context, string, string) error {
	f.call()

	return f.err
}

type gqlResponse struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

func (r gqlResponse) code(t *testing.T) any {
	t.Helper()
	require.NotEmpty(t, r.Errors)

	return r.Errors[0].Extensions["code"]
}

func do(t *testing.T, tk usecase.Task, authenticated bool, query string, vars map[string]any) (resp gqlResponse, body string) {
	t.Helper()

	manager := jwt.New("secret", time.Hour)

	payload, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	if authenticated {
		token, err := manager.GenerateToken(_userID)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	graphql.NewHandler(tk, manager, noopLogger{}).ServeHTTP(rec, req)

	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp), rec.Body.String())

	return resp, rec.Body.String()
}

const (
	_createTask = `mutation($input: CreateTaskInput!) { createTask(input: $input) { id userId title status } }`
	_getTask    = `query { task(id: "task-1") { id title status } }`
)

func TestCreateTask(t *testing.T) {
	t.Parallel()

	resp, _ := do(t, &fakeTask{}, true, _createTask, map[string]any{"input": map[string]any{"title": "Write docs"}})

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"id":"task-1","userId":"user-1","title":"Write docs","status":"TODO"}`,
		string(resp.Data["createTask"]))
}

func TestCreateTaskTitleTooLong(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{}
	title := strings.Repeat("x", 256)

	resp, body := do(t, tk, true, _createTask, map[string]any{"input": map[string]any{"title": title}})

	assert.Equal(t, "BAD_USER_INPUT", resp.code(t))
	assert.NotContains(t, body, title)
	assert.False(t, tk.called)
}

func TestUnauthenticated(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{}

	resp, _ := do(t, tk, false, _getTask, nil)

	assert.Equal(t, "UNAUTHENTICATED", resp.code(t))
	assert.Equal(t, "unauthorized", resp.Errors[0].Message)
	assert.False(t, tk.called)
}

func TestIntrospectionWithoutToken(t *testing.T) {
	t.Parallel()

	resp, _ := do(t, &fakeTask{}, false, `{ __schema { queryType { name } } }`, nil)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"queryType":{"name":"Query"}}`, string(resp.Data["__schema"]))
}

func TestGetTaskErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"not found", entity.ErrTaskNotFound, "NOT_FOUND"},
		{"forbidden", entity.ErrTaskForbidden, "FORBIDDEN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp, _ := do(t, &fakeTask{err: tc.err}, true, _getTask, nil)

			assert.Equal(t, tc.code, resp.code(t))
		})
	}
}

func TestListTasks(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo}}

	resp, _ := do(t, tk, true, `query { tasks(status: TODO, limit: 5) { total tasks { id status } } }`, nil)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"total":1,"tasks":[{"id":"task-1","status":"TODO"}]}`, string(resp.Data["tasks"]))
	require.NotNil(t, tk.listStatus)
	assert.Equal(t, entity.TaskStatusTodo, *tk.listStatus)
}

func TestListTasksInvalidLimit(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{}

	resp, _ := do(t, tk, true, `query { tasks(limit: 0) { total } }`, nil)

	assert.Equal(t, "BAD_USER_INPUT", resp.code(t))
	assert.False(t, tk.called)
}

func TestUpdateTask(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{stored: entity.Task{ID: "task-1", Status: entity.TaskStatusInProgress}}

	resp, _ := do(t, tk, true,
		`mutation { updateTask(id: "task-1", input: {title: "New", description: "Body"}) { title description status } }`, nil)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"title":"New","description":"Body","status":"IN_PROGRESS"}`, string(resp.Data["updateTask"]))
}

func TestTransitionTask(t *testing.T) {
	t.Parallel()

	const query = `mutation { transitionTask(id: "task-1", status: IN_PROGRESS) { id status } }`

	resp, _ := do(t, &fakeTask{stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo}}, true, query, nil)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"id":"task-1","status":"IN_PROGRESS"}`, string(resp.Data["transitionTask"]))

	resp, _ = do(t, &fakeTask{err: entity.ErrInvalidTransition}, true, query, nil)

	assert.Equal(t, "BAD_USER_INPUT", resp.code(t))
	assert.Equal(t, "invalid status transition", resp.Errors[0].Message)
}

func TestPlaygroundTargetsEndpoint(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/graphql/playground", http.NoBody)

	graphql.NewPlayground("/graphql").ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), `"/graphql"`)
}

func TestDeleteTask(t *testing.T) {
	t.Parallel()

	resp, _ := do(t, &fakeTask{}, true, `mutation { deleteTask(id: "task-1") }`, nil)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `true`, string(resp.Data["deleteTask"]))
}

func TestPanicIsHidden(t *testing.T) {
	t.Parallel()

	resp, body := do(t, &fakeTask{panicMsg: "secret panic detail"}, true, _getTask, nil)

	assert.Equal(t, "INTERNAL", resp.code(t))
	assert.Equal(t, "internal server error", resp.Errors[0].Message)
	assert.NotContains(t, body, "secret panic detail")
}
