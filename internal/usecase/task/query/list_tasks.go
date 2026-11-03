package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// _defaultLimit is the page size ListTasks and ListAllTasks use when the caller asks for none.
const _defaultLimit = 10

// TaskPage is one page of tasks, newest first, and how many match in all.
type TaskPage struct {
	Tasks []entity.Task
	Total int
}

// ListTasks lists UserID's tasks, of Status when it is set. A Limit of 0 or less is the default
// page; a negative Offset is 0.
type ListTasks struct {
	UserID string
	Status *entity.TaskStatus
	Limit  int
	Offset int
}

// QueryName -.
func (ListTasks) QueryName() string { return "ListTasks" }

// ListTasksHandler handles ListTasks.
type ListTasksHandler struct {
	Tasks repo.TaskReadModel
}

// Handle -.
func (h ListTasksHandler) Handle(ctx context.Context, q ListTasks) (TaskPage, error) {
	tasks, total, err := h.Tasks.List(ctx, q.UserID, filter(q.Status, q.Limit, q.Offset))
	if err != nil {
		return TaskPage{}, fmt.Errorf("ListTasks - h.Tasks.List: %w", err)
	}

	return TaskPage{Tasks: tasks, Total: total}, nil
}

// filter applies the paging defaults.
func filter(status *entity.TaskStatus, limit, offset int) repo.TaskFilter {
	if limit <= 0 {
		limit = _defaultLimit
	}

	return repo.TaskFilter{Status: status, Limit: int64(limit), Offset: int64(max(offset, 0))}
}
