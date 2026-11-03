package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Importer turns every users row written before users stored events, with its identities, into its
// stream's first event, UserImported, and claims the user's email, username and identities.
// eventstore.Backfill runs it once.
type Importer struct {
	pg    *postgres.Postgres
	store repo.EventStore
}

var _ repo.Importer = (*Importer)(nil)

// NewImporter returns the user importer, reading pg and saving through store.
func NewImporter(pg *postgres.Postgres, store repo.EventStore) *Importer {
	return &Importer{pg: pg, store: store}
}

// Domain -.
func (*Importer) Domain() string {
	return "user"
}

// Import -.
func (i *Importer) Import(ctx context.Context) error {
	q := queries.New(i.pg.Executor(ctx))

	rows, err := q.ListEveryUser(ctx)
	if err != nil {
		return fmt.Errorf("UserImporter - q.ListEveryUser: %w", err)
	}

	links, err := q.ListEveryIdentity(ctx)
	if err != nil {
		return fmt.Errorf("UserImporter - q.ListEveryIdentity: %w", err)
	}

	identities := make(map[string][]entity.UserIdentity, len(links))
	for _, l := range links {
		identities[l.UserID] = append(identities[l.UserID],
			entity.UserIdentity{Issuer: l.Issuer, Subject: l.Subject, CreatedAt: l.CreatedAt})
	}

	users := make([]entity.User, len(rows))
	for j := range rows {
		users[j] = entity.User(rows[j])
	}

	return ImportUsers(ctx, i.store, users, identities)
}

// ImportUsers saves each row, with its identities by user ID, as a new stream holding UserImported,
// and claims what the user holds. A user that has a stream already keeps it.
func ImportUsers(ctx context.Context, store repo.EventStore, rows []entity.User, identities map[string][]entity.UserIdentity) error {
	for i := range rows {
		user, claims := entity.ImportUserAggregate(&rows[i], identities[rows[i].ID])

		_, err := store.Save(ctx, user, claims...)
		if err != nil && !errors.Is(err, entity.ErrConcurrencyConflict) {
			return fmt.Errorf("UserImporter - import %s: %w", rows[i].ID, err)
		}
	}

	return nil
}
