package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// ListAllTasks lists every user's tasks, as ListTasks pages them. Only a caller holding
// entity.RoleAdmin may send it: the query bus's authorize step stops anyone else with
// entity.ErrAdminRequired.
type ListAllTasks struct {
	cqrs.Query[entity.List[entity.Task]]
	entity.Page

	Principal entity.Principal
	Status    *entity.TaskStatus
}

// AdminOnly -.
func (ListAllTasks) AdminOnly() {}

// Caller -.
func (q ListAllTasks) Caller() entity.Principal { return q.Principal }

// ListAllTasksHandler handles ListAllTasks.
type ListAllTasksHandler struct {
	Tasks repo.TaskReadModel
}

// Handle -.
func (h ListAllTasksHandler) Handle(ctx context.Context, q ListAllTasks) (entity.List[entity.Task], error) {
	tasks, err := h.Tasks.ListAll(ctx, q.Status, q.Page)
	if err != nil {
		return entity.List[entity.Task]{}, fmt.Errorf("ListAllTasks - h.Tasks.ListAll: %w", err)
	}

	return tasks, nil
}
