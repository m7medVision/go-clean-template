package projection

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Projector is a subscriber that writes nothing but its own read-model tables, so writes wait for
// it and it can be rebuilt from the first event.
type Projector interface {
	repo.Subscriber
	// Tables are what a rebuild empties.
	Tables() []string
}

// ProjectFunc applies one event to a read model's tables through q. An event it has no case for is
// entity.ErrUnhandledEvent.
type ProjectFunc func(ctx context.Context, q queries.Querier, e *entity.RecordedEvent) error

type projector struct {
	name    string
	tables  []string
	handles []entity.Event
	pg      *postgres.Postgres
	project ProjectFunc
}

// NewProjector returns the projector called name, which keeps tables up to date from the events of
// the types handles lists: project applies each one through queries that write in the transaction
// the runner's context carries, so the rows commit with the checkpoint's move.
func NewProjector(name string, tables []string, handles []entity.Event, pg *postgres.Postgres, project ProjectFunc) Projector {
	return &projector{name: name, tables: tables, handles: handles, pg: pg, project: project}
}

func (p *projector) Name() string {
	return p.name
}

func (p *projector) Tables() []string {
	return p.tables
}

func (p *projector) Handles() []entity.Event {
	return p.handles
}

func (p *projector) Handle(ctx context.Context, e *entity.RecordedEvent) error {
	if err := p.project(ctx, queries.New(p.pg.Executor(ctx)), e); err != nil {
		return fmt.Errorf("projection - %s projector: %w", p.name, err)
	}

	return nil
}
