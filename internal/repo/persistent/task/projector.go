package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// NewProjector keeps the tasks table, the task read model, up to date from the task events. The
// worker runs it.
func NewProjector(pg *postgres.Postgres) projection.Projector {
	return projection.NewProjector("task", []string{"tasks"}, entity.TaskEvents(), pg, project)
}

// project applies one event to the tasks table through q.
func project(ctx context.Context, q queries.Querier, e *entity.RecordedEvent) error {
	switch ev := e.Event.(type) {
	case entity.TaskCreated:
		return q.InsertTaskRow(ctx, queries.InsertTaskRowParams{
			ID: ev.TaskID, UserID: ev.UserID, Title: ev.Title, Description: ev.Description,
			Status: entity.TaskStatusTodo, CreatedAt: ev.At, UpdatedAt: ev.At,
		})
	case entity.TaskImported:
		return q.InsertTaskRow(ctx, queries.InsertTaskRowParams{
			ID: ev.TaskID, UserID: ev.UserID, Title: ev.Title, Description: ev.Description,
			Status: ev.Status, CreatedAt: ev.CreatedAt, UpdatedAt: ev.UpdatedAt,
		})
	case entity.TaskUpdated:
		return q.SetTaskText(ctx, queries.SetTaskTextParams{
			ID: ev.TaskID, Title: ev.Title, Description: ev.Description, UpdatedAt: ev.At,
		})
	case entity.TaskTransitioned:
		return q.SetTaskStatus(ctx, queries.SetTaskStatusParams{ID: ev.TaskID, Status: ev.To, UpdatedAt: ev.At})
	case entity.TaskDeleted:
		return q.DeleteTaskRow(ctx, ev.TaskID)
	default:
		return entity.ErrUnhandledEvent
	}
}
