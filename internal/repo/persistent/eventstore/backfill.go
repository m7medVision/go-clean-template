package eventstore

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

const _markerPrefix = "_backfill/"

// Importer turns the rows one domain wrote before it stored events into events, once (see
// Backfill). Import runs inside the backfill's transaction: it saves through the event store,
// which joins it.
type Importer interface {
	Domain() string
	Import(ctx context.Context) error
}

// Backfill runs each importer that has not run before: it turns its domain's rows, written before
// the domain stored events, into events. Each importer runs in one transaction with its marker
// (_backfill/<domain>), so it either imports everything and is marked done, or nothing and runs
// again next start. The transaction holds the store's append lock, so with several app replicas
// one imports and the others then find the marker.
func Backfill(ctx context.Context, pg *postgres.Postgres, importers ...Importer) error {
	for _, imp := range importers {
		if err := pg.InTx(ctx, func(ctx context.Context) error { return backfill(ctx, pg, imp) }); err != nil {
			return fmt.Errorf("eventstore - Backfill %s: %w", imp.Domain(), err)
		}
	}

	return nil
}

func backfill(ctx context.Context, pg *postgres.Postgres, imp Importer) error {
	q := queries.New(pg.Executor(ctx))
	marker := _markerPrefix + imp.Domain()

	if err := q.LockEventStore(ctx, _lockKey); err != nil {
		return fmt.Errorf("q.LockEventStore: %w", err)
	}

	done, err := q.HasMarker(ctx, marker)
	if err != nil {
		return fmt.Errorf("q.HasMarker: %w", err)
	}

	if done {
		return nil
	}

	if err := imp.Import(ctx); err != nil {
		return err
	}

	if err := q.PutMarker(ctx, marker); err != nil {
		return fmt.Errorf("q.PutMarker: %w", err)
	}

	return nil
}
