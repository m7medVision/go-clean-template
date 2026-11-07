package task

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Importer turns every tasks row written before tasks stored events into its stream's first
// event, TaskImported. eventstore.Backfill runs it once.
type Importer struct {
	pg *postgres.Postgres
}

var _ eventstore.Importer = (*Importer)(nil)

// NewImporter returns the task importer, reading pg.
func NewImporter(pg *postgres.Postgres) *Importer {
	return &Importer{pg: pg}
}

// Domain -.
func (*Importer) Domain() string {
	return "task"
}

// Import -.
func (i *Importer) Import(ctx context.Context) ([]eventstore.Import, error) {
	return imports(ctx, queries.New(i.pg.Executor(ctx)))
}

func imports(ctx context.Context, q queries.Querier) ([]eventstore.Import, error) {
	rows, err := q.ListEveryTask(ctx)
	if err != nil {
		return nil, fmt.Errorf("TaskImporter - q.ListEveryTask: %w", err)
	}

	imports := make([]eventstore.Import, len(rows))
	for j := range rows {
		task := entity.Task(rows[j])
		imports[j] = eventstore.Import{Aggregate: entity.ImportTaskAggregate(&task)}
	}

	return imports, nil
}
