package exchange

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Importer turns every exchange_history row stored before conversions stored events into a
// conversion stream holding ConversionImported. A row with no conversion_id gets one first, so the
// row and its stream share it. eventstore.Backfill runs it once.
type Importer struct {
	pg *postgres.Postgres
}

var _ eventstore.Importer = (*Importer)(nil)

// NewImporter returns the exchange importer, reading pg.
func NewImporter(pg *postgres.Postgres) *Importer {
	return &Importer{pg: pg}
}

// Domain -.
func (*Importer) Domain() string {
	return "exchange"
}

// Import -.
func (i *Importer) Import(ctx context.Context) ([]eventstore.Import, error) {
	return imports(ctx, queries.New(i.pg.Executor(ctx)))
}

// imports returns the conversions in the order the rows were stored, so a rebuild keeps it.
func imports(ctx context.Context, q queries.Querier) ([]eventstore.Import, error) {
	rows, err := q.ListEveryExchange(ctx)
	if err != nil {
		return nil, fmt.Errorf("ExchangeImporter - q.ListEveryExchange: %w", err)
	}

	imports := make([]eventstore.Import, len(rows))

	for j := range rows {
		row := &rows[j]

		id := uuid.NewString()
		if row.ConversionID != nil {
			id = *row.ConversionID
		} else if err = q.SetConversionID(ctx, queries.SetConversionIDParams{ID: row.ID, ConversionID: &id}); err != nil {
			return nil, fmt.Errorf("ExchangeImporter - q.SetConversionID: %w", err)
		}

		c := entity.Conversion{
			From: row.FromCurrency, To: row.ToCurrency, Amount: row.Amount, Rate: row.Rate,
			Result: row.Result, RateDate: row.RateDate,
		}
		imports[j] = eventstore.Import{Aggregate: entity.ImportConversionAggregate(id, row.UserID, &c, row.CreatedAt)}
	}

	return imports, nil
}
