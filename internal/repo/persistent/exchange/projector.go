package exchange

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// NewProjector keeps exchange_history, the exchange read model, up to date from the conversion
// events. The worker runs it.
func NewProjector(pg *postgres.Postgres) projection.Projector {
	return projection.NewProjector("exchange", []string{"exchange_history"}, entity.ConversionEvents(), pg, project)
}

// project applies one event to exchange_history through q.
func project(ctx context.Context, q queries.Querier, e *entity.RecordedEvent) error {
	switch ev := e.Event.(type) {
	case entity.CurrencyConverted:
		return q.InsertConversionRow(ctx, queries.InsertConversionRowParams{
			ConversionID: &ev.ConversionID, UserID: ev.UserID, FromCurrency: ev.From, ToCurrency: ev.To,
			Amount: ev.Amount, Rate: ev.Rate, Result: ev.Result, RateDate: ev.RateDate, CreatedAt: ev.At,
		})
	case entity.ConversionImported:
		return q.InsertConversionRow(ctx, queries.InsertConversionRowParams{
			ConversionID: &ev.ConversionID, UserID: ev.UserID, FromCurrency: ev.From, ToCurrency: ev.To,
			Amount: ev.Amount, Rate: ev.Rate, Result: ev.Result, RateDate: ev.RateDate, CreatedAt: ev.CreatedAt,
		})
	default:
		return entity.ErrUnhandledEvent
	}
}
