package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// ListAllTasks lists every user's tasks, as ListTasks pages them. Only a caller holding
// entity.RoleAdmin may send it: the query bus's authorize step stops anyone else with
// entity.ErrAdminRequired.
type ListAllTasks struct {
	cqrs.Query[entity.List[mongotask.TaskWithAttributes]]
	entity.Page

	Principal entity.Principal
	Status    *entity.TaskStatus
}

func (ListAllTasks) AdminOnly() {}

func (q ListAllTasks) Caller() entity.Principal { return q.Principal }

func (q ListAllTasks) Validate() error { return validateList(q.Status, q.Page) }

type ListAllTasksHandler struct {
	Tasks      repo.TaskReadModel
	Attributes repo.TaskAttributesStore
}

func (h ListAllTasksHandler) Handle(ctx context.Context, q ListAllTasks) (entity.List[mongotask.TaskWithAttributes], error) {
	tasks, err := h.Tasks.ListAll(ctx, q.Status, q.Page)
	if err != nil {
		return entity.List[mongotask.TaskWithAttributes]{}, fmt.Errorf("ListAllTasks - h.Tasks.ListAll: %w", err)
	}

	return withAttributes(ctx, h.Attributes, tasks, "ListAllTasks")
}
