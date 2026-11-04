package task

import (
	"context"
	"errors"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Importer turns every tasks row written before tasks stored events into its stream's first
// event, TaskImported. eventstore.Backfill runs it once.
type Importer struct {
	pg    *postgres.Postgres
	store repo.EventStore
}

var _ repo.Importer = (*Importer)(nil)

// NewImporter returns the task importer, reading pg and saving through store.
func NewImporter(pg *postgres.Postgres, store repo.EventStore) *Importer {
	return &Importer{pg: pg, store: store}
}

// Domain -.
func (*Importer) Domain() string {
	return "task"
}

// Import -.
func (i *Importer) Import(ctx context.Context) error {
	rows, err := queries.New(i.pg.Executor(ctx)).ListEveryTask(ctx)
	if err != nil {
		return fmt.Errorf("TaskImporter - q.ListEveryTask: %w", err)
	}

	tasks := make([]entity.Task, len(rows))
	for j := range rows {
		tasks[j] = entity.Task(rows[j])
	}

	return ImportTasks(ctx, i.store, tasks)
}

// ImportTasks saves each row as a new stream holding TaskImported. A task that has a stream already
// keeps it: its row came from its events.
func ImportTasks(ctx context.Context, store repo.EventStore, rows []entity.Task) error {
	for i := range rows {
		_, err := store.Save(ctx, entity.ImportTaskAggregate(&rows[i]))
		if err != nil && !errors.Is(err, entity.ErrConcurrencyConflict) {
			return fmt.Errorf("TaskImporter - import %s: %w", rows[i].ID, err)
		}
	}

	return nil
}
