package exchange

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Projector keeps exchange_history, the exchange read model, up to date from the conversion
// events. The worker runs it; it writes through the transaction the runner's context carries.
type Projector struct {
	pg *postgres.Postgres
}

var _ repo.Subscriber = (*Projector)(nil)

func NewProjector(pg *postgres.Postgres) *Projector {
	return &Projector{pg: pg}
}

func (*Projector) Name() string {
	return "exchange"
}

// Tables are what a rebuild empties.
func (*Projector) Tables() []string {
	return []string{"exchange_history"}
}

func (*Projector) Handles() []entity.Event {
	return entity.ConversionEvents()
}

func (p *Projector) Handle(ctx context.Context, e *entity.RecordedEvent) error {
	return Project(ctx, queries.New(p.pg.Executor(ctx)), e)
}

// Project applies one event to exchange_history through q.
func Project(ctx context.Context, q queries.Querier, e *entity.RecordedEvent) error {
	var err error

	switch ev := e.Event.(type) {
	case entity.CurrencyConverted:
		err = q.InsertConversionRow(ctx, queries.InsertConversionRowParams{
			ConversionID: &ev.ConversionID, UserID: ev.UserID, FromCurrency: ev.From, ToCurrency: ev.To,
			Amount: ev.Amount, Rate: ev.Rate, Result: ev.Result, RateDate: ev.RateDate, CreatedAt: ev.At,
		})
	case entity.ConversionImported:
		err = q.InsertConversionRow(ctx, queries.InsertConversionRowParams{
			ConversionID: &ev.ConversionID, UserID: ev.UserID, FromCurrency: ev.From, ToCurrency: ev.To,
			Amount: ev.Amount, Rate: ev.Rate, Result: ev.Result, RateDate: ev.RateDate, CreatedAt: ev.CreatedAt,
		})
	default:
		err = entity.ErrUnhandledEvent
	}

	if err != nil {
		return fmt.Errorf("ExchangeProjector - %s at %d: %w", e.Event.EventType(), e.Position, err)
	}

	return nil
}
