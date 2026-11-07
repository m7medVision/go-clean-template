package eventstore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// ErrImportPending is a change that would lose rows an importer has still to turn into events: a
// rebuild empties the tables the importers read.
var ErrImportPending = errors.New("eventstore: an import has not run yet")

// Importer reads the rows one domain wrote before it stored events (see Backfill).
type Importer interface {
	Domain() string
	// Import returns one new stream per row, holding the row's *Imported event, with the claims
	// the row holds. It runs inside the backfill's transaction, and anything it writes joins it.
	Import(ctx context.Context) ([]Import, error)
}

// Import is one stream an importer creates: a new aggregate holding its *Imported event, and the
// claims to reserve with it.
type Import struct {
	Aggregate entity.Aggregate
	Claims    []entity.Claim
}

// Backfill runs each importer that has not run before and saves the streams it returns, skipping a
// stream that exists already: its row came from its events. Each importer runs in one transaction
// with its marker in backfill_markers, so it either imports everything and is marked done, or
// nothing and runs again next start. The transaction holds the store's append lock, so with several
// app replicas one imports and the others then find the marker, and no stream can appear between
// the check and the save.
func Backfill(ctx context.Context, pg *postgres.Postgres, store repo.EventStore, importers ...Importer) error {
	// A database upgraded from v0.3 keeps its markers among the checkpoints.
	if err := queries.New(pg.Executor(ctx)).MoveLegacyMarkers(ctx); err != nil {
		return fmt.Errorf("eventstore - Backfill - q.MoveLegacyMarkers: %w", err)
	}

	for _, imp := range importers {
		err := pg.InTx(ctx, func(ctx context.Context) error { return backfill(ctx, pg, store, imp) })
		if err != nil {
			return fmt.Errorf("eventstore - Backfill %s: %w", imp.Domain(), err)
		}
	}

	return nil
}

func backfill(ctx context.Context, pg *postgres.Postgres, store repo.EventStore, imp Importer) error {
	q := queries.New(pg.Executor(ctx))

	if err := q.LockEventStore(ctx, _lockKey); err != nil {
		return fmt.Errorf("q.LockEventStore: %w", err)
	}

	done, err := q.HasMarker(ctx, imp.Domain())
	if err != nil {
		return fmt.Errorf("q.HasMarker: %w", err)
	}

	if done {
		return nil
	}

	imports, err := imp.Import(ctx)
	if err != nil {
		return err
	}

	for i := range imports {
		if err := save(ctx, q, store, &imports[i]); err != nil {
			return err
		}
	}

	if err = q.PutMarker(ctx, imp.Domain()); err != nil {
		return fmt.Errorf("q.PutMarker: %w", err)
	}

	return nil
}

// save saves im unless its stream exists already. It checks first rather than relying on the
// store's version check, so a stream that exists never makes a statement fail inside the backfill's
// transaction.
func save(ctx context.Context, q *queries.Queries, store repo.EventStore, im *Import) error {
	version, err := q.StreamVersion(ctx, im.Aggregate.StreamID())
	if err != nil {
		return fmt.Errorf("q.StreamVersion: %w", err)
	}

	if version > 0 {
		return nil
	}

	if _, err = store.Save(ctx, im.Aggregate, im.Claims...); err != nil {
		return fmt.Errorf("import %s: %w", im.Aggregate.StreamID(), err)
	}

	return nil
}

// RequireImported fails with ErrImportPending, naming them, while any of importers has not run.
// Once it passes it keeps passing: a marker is written with its import and never removed.
func RequireImported(ctx context.Context, pg *postgres.Postgres, importers ...Importer) error {
	if len(importers) == 0 {
		return nil
	}

	domains := make([]string, len(importers))
	for i, imp := range importers {
		domains[i] = imp.Domain()
	}

	imported, err := queries.New(pg.Executor(ctx)).ImportedDomains(ctx, domains)
	if err != nil {
		return fmt.Errorf("eventstore - RequireImported - q.ImportedDomains: %w", err)
	}

	pending := slices.DeleteFunc(domains, func(d string) bool { return slices.Contains(imported, d) })
	if len(pending) > 0 {
		return fmt.Errorf("%w: %s; start the app once to import", ErrImportPending, strings.Join(pending, ", "))
	}

	return nil
}
