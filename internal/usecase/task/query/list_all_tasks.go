package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// ListAllTasks lists every user's tasks, as ListTasks pages them. Only a caller holding
// entity.RoleAdmin may send it: the query bus's authorize step stops anyone else with
// entity.ErrAdminRequired.
type ListAllTasks struct {
	Principal entity.Principal
	Status    *entity.TaskStatus
	Limit     int
	Offset    int
}

// QueryName -.
func (ListAllTasks) QueryName() string { return "ListAllTasks" }

// AdminOnly -.
func (ListAllTasks) AdminOnly() {}

// Caller -.
func (q ListAllTasks) Caller() entity.Principal { return q.Principal }

// ListAllTasksHandler handles ListAllTasks.
type ListAllTasksHandler struct {
	Tasks repo.TaskReadModel
}

// Handle -.
func (h ListAllTasksHandler) Handle(ctx context.Context, q ListAllTasks) (TaskPage, error) {
	tasks, total, err := h.Tasks.ListAll(ctx, filter(q.Status, q.Limit, q.Offset))
	if err != nil {
		return TaskPage{}, fmt.Errorf("ListAllTasks - h.Tasks.ListAll: %w", err)
	}

	return TaskPage{Tasks: tasks, Total: total}, nil
}
