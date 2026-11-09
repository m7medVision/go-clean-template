// Package query holds the task domain's queries, one file each, with their handlers. They read
// only the task read model.
package query

import (
	"context"
	"fmt"

	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// GetTask reads UserID's task: entity.ErrTaskNotFound when there is none, entity.ErrTaskForbidden
// when it is another user's.
type GetTask struct {
	cqrs.Query[mongotask.TaskWithAttributes]

	UserID string
	TaskID string
}

type GetTaskHandler struct {
	Tasks      repo.TaskReadModel
	Attributes repo.TaskAttributesStore
}

func (h GetTaskHandler) Handle(ctx context.Context, q GetTask) (mongotask.TaskWithAttributes, error) {
	task, err := h.Tasks.Get(ctx, q.TaskID)
	if err != nil {
		return mongotask.TaskWithAttributes{}, fmt.Errorf("GetTask - h.Tasks.Get: %w", err)
	}

	if err = task.ReadableBy(q.UserID); err != nil {
		return mongotask.TaskWithAttributes{}, fmt.Errorf("GetTask: %w", err)
	}

	attrs, err := h.Attributes.Get(ctx, q.TaskID)
	if err != nil {
		return mongotask.TaskWithAttributes{}, fmt.Errorf("GetTask - h.Attributes.Get: %w", err)
	}

	return mongotask.TaskWithAttributes{Task: task, Attributes: attrs}, nil
}
