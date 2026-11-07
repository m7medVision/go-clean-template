package passkey

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Importer turns every webauthn_credentials row stored before passkeys stored events into its
// stream's first event, PasskeyImported. eventstore.Backfill runs it once.
type Importer struct {
	pg *postgres.Postgres
}

var _ eventstore.Importer = (*Importer)(nil)

func NewImporter(pg *postgres.Postgres) *Importer {
	return &Importer{pg: pg}
}

func (*Importer) Domain() string {
	return "passkey"
}

func (i *Importer) Import(ctx context.Context) ([]eventstore.Import, error) {
	return imports(ctx, queries.New(i.pg.Executor(ctx)))
}

func imports(ctx context.Context, q queries.Querier) ([]eventstore.Import, error) {
	rows, err := q.ListEveryPasskey(ctx)
	if err != nil {
		return nil, fmt.Errorf("PasskeyImporter - q.ListEveryPasskey: %w", err)
	}

	imports := make([]eventstore.Import, len(rows))
	for j := range rows {
		passkey := entity.Passkey(rows[j])
		imports[j] = eventstore.Import{Aggregate: entity.ImportPasskeyAggregate(&passkey)}
	}

	return imports, nil
}
