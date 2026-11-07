package passkey

import (
	"context"
	"errors"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Importer turns every webauthn_credentials row stored before passkeys stored events into its
// stream's first event, PasskeyImported. eventstore.Backfill runs it once.
type Importer struct {
	pg    *postgres.Postgres
	store repo.EventStore
}

var _ eventstore.Importer = (*Importer)(nil)

// NewImporter returns the passkey importer, reading pg and saving through store.
func NewImporter(pg *postgres.Postgres, store repo.EventStore) *Importer {
	return &Importer{pg: pg, store: store}
}

// Domain -.
func (*Importer) Domain() string {
	return "passkey"
}

// Import -.
func (i *Importer) Import(ctx context.Context) error {
	rows, err := queries.New(i.pg.Executor(ctx)).ListEveryPasskey(ctx)
	if err != nil {
		return fmt.Errorf("PasskeyImporter - q.ListEveryPasskey: %w", err)
	}

	passkeys := make([]entity.Passkey, len(rows))
	for j := range rows {
		passkeys[j] = entity.Passkey(rows[j])
	}

	return ImportPasskeys(ctx, i.store, passkeys)
}

// ImportPasskeys saves each row as a new stream holding PasskeyImported. A passkey that has a stream
// already keeps it.
func ImportPasskeys(ctx context.Context, store repo.EventStore, rows []entity.Passkey) error {
	for j := range rows {
		_, err := store.Save(ctx, entity.ImportPasskeyAggregate(&rows[j]))
		if err != nil && !errors.Is(err, entity.ErrConcurrencyConflict) {
			return fmt.Errorf("PasskeyImporter - import %s: %w", rows[j].ID, err)
		}
	}

	return nil
}
