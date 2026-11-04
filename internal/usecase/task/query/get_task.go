// Package query holds the task domain's queries, one file each, with their handlers. They read
// only the task read model.
package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// GetTask reads UserID's task: entity.ErrTaskNotFound when there is none, entity.ErrTaskForbidden
// when it is another user's.
type GetTask struct {
	UserID string
	TaskID string
}

// QueryName -.
func (GetTask) QueryName() string { return "GetTask" }

// GetTaskHandler handles GetTask.
type GetTaskHandler struct {
	Tasks repo.TaskReadModel
}

// Handle -.
func (h GetTaskHandler) Handle(ctx context.Context, q GetTask) (entity.Task, error) {
	task, err := h.Tasks.Get(ctx, q.UserID, q.TaskID)
	if err != nil {
		return entity.Task{}, fmt.Errorf("GetTask - h.Tasks.Get: %w", err)
	}

	return task, nil
}
