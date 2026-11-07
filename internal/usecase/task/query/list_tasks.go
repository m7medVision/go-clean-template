package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// ListTasks lists one page of UserID's tasks, of Status when it is set, newest first.
type ListTasks struct {
	cqrs.Query[entity.List[entity.Task]]
	entity.Page

	UserID string
	Status *entity.TaskStatus
}

// ListTasksHandler handles ListTasks.
type ListTasksHandler struct {
	Tasks repo.TaskReadModel
}

// Handle -.
func (h ListTasksHandler) Handle(ctx context.Context, q ListTasks) (entity.List[entity.Task], error) {
	tasks, err := h.Tasks.List(ctx, q.UserID, q.Status, q.Page)
	if err != nil {
		return entity.List[entity.Task]{}, fmt.Errorf("ListTasks - h.Tasks.List: %w", err)
	}

	return tasks, nil
}
