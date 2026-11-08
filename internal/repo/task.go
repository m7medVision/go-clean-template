package repo

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
)

//go:generate mockgen -source=task.go -destination=repomock/task.go -package=repomock

// TaskReadModel reads the tasks table, which only the task projector writes. Get of a missing
// task, or of an ID that is not a UUID, is entity.ErrTaskNotFound; whether the caller may read it
// is entity.Task's rule.
type TaskReadModel interface {
	Get(ctx context.Context, taskID string) (entity.Task, error)
	// List is one page of the user's tasks, of status when it is set, newest first.
	List(ctx context.Context, userID string, status *entity.TaskStatus, page entity.Page) (entity.List[entity.Task], error)
	// ListAll is List across every user.
	ListAll(ctx context.Context, status *entity.TaskStatus, page entity.Page) (entity.List[entity.Task], error)
}

// TaskAttributesStore keeps each task's attributes, one document per task, outside the event store:
// written directly, with no events and no history. A task with no document has empty attributes.
type TaskAttributesStore interface {
	Get(ctx context.Context, taskID string) (mongotask.Attributes, error)
	// GetMany is the attributes of each task that has a document, by task ID.
	GetMany(ctx context.Context, taskIDs []string) (map[string]mongotask.Attributes, error)
	// Patch sets and clears the fields the patch names and leaves the rest, creating the document
	// when there is none.
	Patch(ctx context.Context, taskID string, p mongotask.AttributesPatch) error
	// Delete removes the task's document; a missing one is no error.
	Delete(ctx context.Context, taskID string) error
}
