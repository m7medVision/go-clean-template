package repo

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

//go:generate mockgen -source=task.go -destination=repomock/task.go -package=repomock

// TaskReadModel reads the tasks table, which only the task projector writes. Get of a missing
// task is entity.ErrTaskNotFound; whether the caller may read it is entity.Task's rule.
type TaskReadModel interface {
	Get(ctx context.Context, taskID string) (entity.Task, error)
	// List is one page of the user's tasks, of status when it is set, newest first.
	List(ctx context.Context, userID string, status *entity.TaskStatus, page entity.Page) (entity.List[entity.Task], error)
	// ListAll is List across every user.
	ListAll(ctx context.Context, status *entity.TaskStatus, page entity.Page) (entity.List[entity.Task], error)
}
