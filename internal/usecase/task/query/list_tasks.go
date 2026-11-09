package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// ListTasks lists one page of UserID's tasks, of Status when it is set, newest first.
type ListTasks struct {
	cqrs.Query[entity.List[mongotask.TaskWithAttributes]]
	entity.Page

	UserID string
	Status *entity.TaskStatus
}

func (q ListTasks) Validate() error { return validateList(q.Status, q.Page) }

type ListTasksHandler struct {
	Tasks      repo.TaskReadModel
	Attributes repo.TaskAttributesStore
}

func (h ListTasksHandler) Handle(ctx context.Context, q ListTasks) (entity.List[mongotask.TaskWithAttributes], error) {
	tasks, err := h.Tasks.List(ctx, q.UserID, q.Status, q.Page)
	if err != nil {
		return entity.List[mongotask.TaskWithAttributes]{}, fmt.Errorf("ListTasks - h.Tasks.List: %w", err)
	}

	return withAttributes(ctx, h.Attributes, tasks, "ListTasks")
}

func withAttributes(
	ctx context.Context, store repo.TaskAttributesStore, tasks entity.List[entity.Task], op string,
) (entity.List[mongotask.TaskWithAttributes], error) {
	ids := make([]string, len(tasks.Items))
	for i := range tasks.Items {
		ids[i] = tasks.Items[i].ID
	}

	attrs, err := store.GetMany(ctx, ids)
	if err != nil {
		return entity.List[mongotask.TaskWithAttributes]{}, fmt.Errorf("%s - store.GetMany: %w", op, err)
	}

	items := make([]mongotask.TaskWithAttributes, len(tasks.Items))
	for i := range tasks.Items {
		items[i] = mongotask.TaskWithAttributes{Task: tasks.Items[i], Attributes: attrs[tasks.Items[i].ID]}
	}

	return entity.List[mongotask.TaskWithAttributes]{Items: items, Total: tasks.Total}, nil
}

func validateList(status *entity.TaskStatus, page entity.Page) error {
	if status != nil {
		if err := entity.ValidateTaskStatus(*status); err != nil {
			return err
		}
	}

	return page.Validate()
}
