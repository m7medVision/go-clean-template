package task_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateTaskAttributesPassesEachFieldsStateThrough(t *testing.T) {
	t.Parallel()

	high := mongotask.PriorityHigh
	tk := &fakeTask{stored: entity.Task{ID: "task-1"}, attrs: mongotask.Attributes{Priority: &high}}

	resp := do(t, tk, true,
		`mutation { updateTaskAttributes(id: "task-1", input: {priority: HIGH, estimate: null}) { id attributes { priority labels } } }`, nil)

	require.Empty(t, resp.Errors)
	assert.Equal(t, mongotask.AttributesPatch{
		Priority: mongotask.SetTo(mongotask.PriorityHigh),
		Estimate: mongotask.Cleared[float64](),
	}, tk.gotPatch)
	assert.JSONEq(t, `{"id":"task-1","attributes":{"priority":"HIGH","labels":null}}`, string(resp.Data["updateTaskAttributes"]))
}

func TestUpdateTaskAttributesInvalidIsBadUserInput(t *testing.T) {
	t.Parallel()

	resp := do(t, &fakeTask{err: entity.ErrInvalidTaskEstimate}, true,
		`mutation { updateTaskAttributes(id: "task-1", input: {estimate: 900}) { id } }`, nil)

	require.NotEmpty(t, resp.Errors)
	assert.Equal(t, "BAD_USER_INPUT", resp.Errors[0].Extensions["code"])
}
