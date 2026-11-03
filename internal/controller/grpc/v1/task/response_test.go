package task_test

import (
	"testing"
	"time"

	grpctask "github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/task"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTaskResponse(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 2, 10, 8, 0, 0, 0, time.UTC)
	task := &entity.Task{
		ID:          "task-id-789",
		UserID:      "user-id-123",
		Title:       "My Task",
		Description: "Task description",
		Status:      entity.TaskStatusInProgress,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	resp := grpctask.NewTaskResponse(task)

	require.NotNil(t, resp)
	assert.Equal(t, task.ID, resp.Id)
	assert.Equal(t, task.UserID, resp.UserId)
	assert.Equal(t, task.Title, resp.Title)
	assert.Equal(t, task.Description, resp.Description)
	assert.Equal(t, string(task.Status), resp.Status)
	assert.Equal(t, "2026-02-10T08:00:00Z", resp.CreatedAt)
	assert.Equal(t, "2026-02-10T08:00:00Z", resp.UpdatedAt)
}

func TestNewListTasksResponse(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := []entity.Task{
		{
			ID:        "task-1",
			UserID:    "user-id-123",
			Title:     "Task One",
			Status:    entity.TaskStatusTodo,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "task-2",
			UserID:    "user-id-123",
			Title:     "Task Two",
			Status:    entity.TaskStatusDone,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	resp := grpctask.NewListTasksResponse(tasks, 2)

	require.NotNil(t, resp)
	assert.Len(t, resp.Tasks, 2)
	assert.Equal(t, int32(2), resp.Total)
	assert.Equal(t, "task-1", resp.Tasks[0].Id)
	assert.Equal(t, "task-2", resp.Tasks[1].Id)
}

func TestNewListTasksResponse_Empty(t *testing.T) {
	t.Parallel()

	resp := grpctask.NewListTasksResponse([]entity.Task{}, 0)

	require.NotNil(t, resp)
	assert.Empty(t, resp.Tasks)
	assert.Equal(t, int32(0), resp.Total)
}
