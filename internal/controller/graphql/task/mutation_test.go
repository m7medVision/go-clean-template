package task_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _createTask = `mutation($input: CreateTaskInput!) { createTask(input: $input) { id userId title status } }`

func TestCreateTask(t *testing.T) {
	t.Parallel()

	resp := do(t, &fakeTask{}, true, _createTask, map[string]any{"input": map[string]any{"title": "Write docs"}})

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"id":"task-1","userId":"user-1","title":"Write docs","status":"TODO"}`,
		string(resp.Data["createTask"]))
}

func TestUpdateTask(t *testing.T) {
	t.Parallel()

	tk := &fakeTask{stored: entity.Task{ID: "task-1", Status: entity.TaskStatusInProgress}}

	resp := do(t, tk, true,
		`mutation { updateTask(id: "task-1", input: {title: "New", description: "Body"}) { title description status } }`, nil)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"title":"New","description":"Body","status":"IN_PROGRESS"}`, string(resp.Data["updateTask"]))
}

func TestTransitionTask(t *testing.T) {
	t.Parallel()

	const query = `mutation { transitionTask(id: "task-1", status: IN_PROGRESS) { id status } }`

	resp := do(t, &fakeTask{stored: entity.Task{ID: "task-1", Status: entity.TaskStatusTodo}}, true, query, nil)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"id":"task-1","status":"IN_PROGRESS"}`, string(resp.Data["transitionTask"]))

	// gqlerr's tests own which code each kind of failure gets; this checks that a resolver's failure
	// reaches the response with its code and message.
	resp = do(t, &fakeTask{err: entity.ErrInvalidTransition}, true, query, nil)

	assert.Equal(t, "BAD_USER_INPUT", resp.Code(t))
	assert.Equal(t, "invalid status transition", resp.Errors[0].Message)
}

func TestDeleteTask(t *testing.T) {
	t.Parallel()

	resp := do(t, &fakeTask{}, true, `mutation { deleteTask(id: "task-1") }`, nil)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `true`, string(resp.Data["deleteTask"]))
}
