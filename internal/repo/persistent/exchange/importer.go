package exchange

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Importer turns every exchange_history row stored before conversions stored events into a
// conversion stream holding ConversionImported. A row with no conversion_id gets one first, so the
// row and its stream share it. eventstore.Backfill runs it once.
type Importer struct {
	pg    *postgres.Postgres
	store repo.EventStore
}

var _ repo.Importer = (*Importer)(nil)

// ImportedConversion is one row to import: the conversion, whose it is, its ID and when it was made.
type ImportedConversion struct {
	ID         string
	UserID     string
	Conversion entity.Conversion
	CreatedAt  time.Time
}

// NewImporter returns the exchange importer, reading pg and saving through store.
func NewImporter(pg *postgres.Postgres, store repo.EventStore) *Importer {
	return &Importer{pg: pg, store: store}
}

// Domain -.
func (*Importer) Domain() string {
	return "exchange"
}

// Import -.
func (i *Importer) Import(ctx context.Context) error {
	q := queries.New(i.pg.Executor(ctx))

	rows, err := q.ListEveryExchange(ctx)
	if err != nil {
		return fmt.Errorf("ExchangeImporter - q.ListEveryExchange: %w", err)
	}

	conversions := make([]ImportedConversion, 0, len(rows))

	for j := range rows {
		id := uuid.NewString()
		if rows[j].ConversionID != nil {
			id = *rows[j].ConversionID
		} else if err = q.SetConversionID(ctx, queries.SetConversionIDParams{ID: rows[j].ID, ConversionID: &id}); err != nil {
			return fmt.Errorf("ExchangeImporter - q.SetConversionID: %w", err)
		}

		conversions = append(conversions, ImportedConversion{
			ID: id, UserID: rows[j].UserID, CreatedAt: rows[j].CreatedAt,
			Conversion: entity.Conversion{
				From: rows[j].FromCurrency, To: rows[j].ToCurrency, Amount: rows[j].Amount, Rate: rows[j].Rate,
				Result: rows[j].Result, RateDate: rows[j].RateDate,
			},
		})
	}

	return ImportConversions(ctx, i.store, conversions)
}

// ImportConversions saves each conversion as a new stream holding ConversionImported, in order. A
// conversion that has a stream already keeps it.
func ImportConversions(ctx context.Context, store repo.EventStore, conversions []ImportedConversion) error {
	for i := range conversions {
		c := &conversions[i]

		_, err := store.Save(ctx, entity.ImportConversionAggregate(c.ID, c.UserID, &c.Conversion, c.CreatedAt))
		if err != nil && !errors.Is(err, entity.ErrConcurrencyConflict) {
			return fmt.Errorf("ExchangeImporter - import %s: %w", c.ID, err)
		}
	}

	return nil
}
