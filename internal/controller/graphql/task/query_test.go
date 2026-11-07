package task_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/graphqltest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _getTask = `query { task(id: "task-1") { id title status } }`

func TestUnauthenticated(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{}

	resp := do(t, tk, false, _getTask, nil)

	assert.Equal(t, "UNAUTHENTICATED", resp.Code(t))
	assert.Equal(t, "unauthorized", resp.Errors[0].Message)
	assert.False(t, tk.called)
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

			resp := do(t, &fakeTask{err: tc.err}, true, _getTask, nil)

			assert.Equal(t, tc.code, resp.Code(t))
		})
	}
}

func TestListTasks(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo}}

	resp := do(t, tk, true, `query { tasks(status: TODO, limit: 5) { total tasks { id status } } }`, nil)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"total":1,"tasks":[{"id":"task-1","status":"TODO"}]}`, string(resp.Data["tasks"]))
	require.NotNil(t, tk.listStatus)
	assert.Equal(t, entity.TaskStatusTodo, *tk.listStatus)
}

func TestAllTasks(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{stored: entity.Task{ID: "task-1", UserID: "bob", Status: entity.TaskStatusDone}}

	resp := doAs(t, tk, []string{"admin"}, `query { allTasks(status: DONE, limit: 5) { total tasks { id userId } } }`)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"total":1,"tasks":[{"id":"task-1","userId":"bob"}]}`, string(resp.Data["allTasks"]))
	assert.Equal(t, entity.Principal{UserID: graphqltest.UserID, Roles: []entity.Role{entity.RoleAdmin}}, tk.listCaller)
	require.NotNil(t, tk.listStatus)
	assert.Equal(t, entity.TaskStatusDone, *tk.listStatus)
}

func TestAllTasksAdminRequired(t *testing.T) {
	t.Parallel()

	resp := doAs(t, &fakeTask{err: entity.ErrAdminRequired}, []string{"user"}, `query { allTasks { total } }`)

	assert.Equal(t, "FORBIDDEN", resp.Code(t))
}

func TestAllTasksUnauthenticated(t *testing.T) {
	t.Parallel()

	resp := do(t, &fakeTask{}, false, `query { allTasks { total } }`, nil)

	assert.Equal(t, "UNAUTHENTICATED", resp.Code(t))
}
