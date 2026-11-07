package projection

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// ErrUnknownProjector is a rebuild of a name no projector has. An event handler is never rebuilt:
// replaying it would repeat its side effects, such as sending every welcome email again.
var ErrUnknownProjector = errors.New("projection: unknown projector")

// Projector is a subscriber that writes nothing but its own read-model tables, so writes wait for
// it and it can be rebuilt from the first event.
type Projector interface {
	repo.Subscriber
	Tables() []string
}

// Rebuild empties the tables of the projector called name and resets its checkpoint, in one
// transaction, and wakes the worker running it, which then replays every event into them. Until it
// catches up, reads from those tables are incomplete.
//
// It refuses with eventstore.ErrImportPending while any of importers has not run: they read the
// read-model tables, so emptying one would lose the rows they have still to import.
func Rebuild(
	ctx context.Context, pg *postgres.Postgres, name string, projectors []Projector, importers []eventstore.Importer,
) error {
	i := slices.IndexFunc(projectors, func(p Projector) bool { return p.Name() == name })
	if i < 0 {
		return fmt.Errorf("%w: %q; known: %s", ErrUnknownProjector, name, strings.Join(Names(projectors), ", "))
	}

	if err := eventstore.RequireImported(ctx, pg, importers...); err != nil {
		return fmt.Errorf("projection - Rebuild %s: %w", name, err)
	}

	return rebuild(ctx, pg, projectors[i])
}

func rebuild(ctx context.Context, pg *postgres.Postgres, p Projector) error {
	tables := make([]string, 0, len(p.Tables()))
	for _, t := range p.Tables() {
		tables = append(tables, pgx.Identifier{t}.Sanitize())
	}

	return pg.InTx(ctx, func(ctx context.Context) error {
		exec := pg.Executor(ctx)

		// The names come from the projector's code and are quoted; TRUNCATE takes no parameters.
		if len(tables) > 0 {
			if _, err := exec.Exec(ctx, "TRUNCATE "+strings.Join(tables, ", ")); err != nil {
				return fmt.Errorf("projection - Rebuild %s - TRUNCATE: %w", p.Name(), err)
			}
		}

		q := queries.New(exec)

		if err := q.ResetCheckpoint(ctx, p.Name()); err != nil {
			return fmt.Errorf("projection - Rebuild %s - ResetCheckpoint: %w", p.Name(), err)
		}

		// Wakes the worker running the projector, so it starts replaying at once.
		if err := q.NotifyEvents(ctx); err != nil {
			return fmt.Errorf("projection - Rebuild %s - NotifyEvents: %w", p.Name(), err)
		}

		return nil
	})
}
