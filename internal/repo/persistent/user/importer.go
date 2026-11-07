package user

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Importer turns every users row written before users stored events, with its identities, into its
// stream's first event, UserImported, and claims the user's email, username and identities.
// eventstore.Backfill runs it once.
type Importer struct {
	pg *postgres.Postgres
}

var _ eventstore.Importer = (*Importer)(nil)

// NewImporter returns the user importer, reading pg.
func NewImporter(pg *postgres.Postgres) *Importer {
	return &Importer{pg: pg}
}

// Domain -.
func (*Importer) Domain() string {
	return "user"
}

// Import -.
func (i *Importer) Import(ctx context.Context) ([]eventstore.Import, error) {
	return imports(ctx, queries.New(i.pg.Executor(ctx)))
}

func imports(ctx context.Context, q queries.Querier) ([]eventstore.Import, error) {
	rows, err := q.ListEveryUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserImporter - q.ListEveryUser: %w", err)
	}

	links, err := q.ListEveryIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserImporter - q.ListEveryIdentity: %w", err)
	}

	identities := make(map[string][]entity.UserIdentity, len(links))
	for _, l := range links {
		identities[l.UserID] = append(identities[l.UserID],
			entity.UserIdentity{Issuer: l.Issuer, Subject: l.Subject, CreatedAt: l.CreatedAt})
	}

	imports := make([]eventstore.Import, len(rows))
	for j := range rows {
		row := entity.User(rows[j])
		user, claims := entity.ImportUserAggregate(&row, identities[row.ID])
		imports[j] = eventstore.Import{Aggregate: user, Claims: claims}
	}

	return imports, nil
}
