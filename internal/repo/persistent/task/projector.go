package task

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Projector keeps the tasks table, the task read model, up to date from the task events. The
// worker runs it; it writes through the transaction the runner's context carries.
type Projector struct {
	pg *postgres.Postgres
}

var _ repo.Subscriber = (*Projector)(nil)

// NewProjector returns the task projector writing to pg.
func NewProjector(pg *postgres.Postgres) *Projector {
	return &Projector{pg: pg}
}

// Name -.
func (*Projector) Name() string {
	return "task"
}

// Tables are what a rebuild empties.
func (*Projector) Tables() []string {
	return []string{"tasks"}
}

// Handle -.
func (p *Projector) Handle(ctx context.Context, e *entity.RecordedEvent) error {
	return Project(ctx, queries.New(p.pg.Executor(ctx)), e)
}

// Project applies one event to the tasks table through q. Events of other domains change nothing.
func Project(ctx context.Context, q queries.Querier, e *entity.RecordedEvent) error {
	var err error

	switch ev := e.Event.(type) {
	case entity.TaskCreated:
		err = q.InsertTaskRow(ctx, queries.InsertTaskRowParams{
			ID: ev.TaskID, UserID: ev.UserID, Title: ev.Title, Description: ev.Description,
			Status: entity.TaskStatusTodo, CreatedAt: ev.At, UpdatedAt: ev.At,
		})
	case entity.TaskImported:
		err = q.InsertTaskRow(ctx, queries.InsertTaskRowParams{
			ID: ev.TaskID, UserID: ev.UserID, Title: ev.Title, Description: ev.Description,
			Status: ev.Status, CreatedAt: ev.CreatedAt, UpdatedAt: ev.UpdatedAt,
		})
	case entity.TaskUpdated:
		err = q.SetTaskText(ctx, queries.SetTaskTextParams{
			ID: ev.TaskID, Title: ev.Title, Description: ev.Description, UpdatedAt: ev.At,
		})
	case entity.TaskTransitioned:
		err = q.SetTaskStatus(ctx, queries.SetTaskStatusParams{ID: ev.TaskID, Status: ev.To, UpdatedAt: ev.At})
	case entity.TaskDeleted:
		err = q.DeleteTaskRow(ctx, ev.TaskID)
	}

	if err != nil {
		return fmt.Errorf("TaskProjector - %s at %d: %w", e.Event.EventType(), e.Position, err)
	}

	return nil
}
