// Package exchange implements the exchange read model on Postgres (exchange_history), the projector
// that writes it from the conversion events, and the importer that turns rows stored before
// conversions stored events into events.
package exchange

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Repo reads exchange_history. Its queries join the transaction the context carries.
type Repo struct {
	pg *postgres.Postgres
}

func NewReadModel(pg *postgres.Postgres) repo.ExchangeReadModel {
	return &Repo{pg: pg}
}

func (r *Repo) q(ctx context.Context) *queries.Queries {
	return queries.New(r.pg.Executor(ctx))
}

// Conversion reads the conversion with conversionID. An ID other than a UUID as the app writes
// them, in lower case, names no conversion, so it is entity.ErrConversionNotFound, not the error
// exchange_history.conversion_id would raise.
func (r *Repo) Conversion(ctx context.Context, conversionID string) (entity.Conversion, error) {
	if id, err := uuid.Parse(conversionID); err != nil || id.String() != conversionID {
		return entity.Conversion{}, entity.ErrConversionNotFound
	}

	row, err := r.q(ctx).GetConversion(ctx, &conversionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Conversion{}, entity.ErrConversionNotFound
		}

		return entity.Conversion{}, fmt.Errorf("ExchangeReadModel - Conversion - r.q(ctx).GetConversion: %w", err)
	}

	return entity.Conversion{
		UserID: row.UserID, From: row.FromCurrency, To: row.ToCurrency, Amount: row.Amount, Rate: row.Rate,
		Result: row.Result, RateDate: row.RateDate,
	}, nil
}

// History returns one page of the user's conversions, newest first, and their total count.
func (r *Repo) History(ctx context.Context, userID string, page entity.Page) (entity.List[entity.Conversion], error) {
	total, err := r.q(ctx).CountExchanges(ctx, userID)
	if err != nil {
		return entity.List[entity.Conversion]{}, fmt.Errorf("ExchangeReadModel - History - r.q(ctx).CountExchanges: %w", err)
	}

	rows, err := r.q(ctx).ListExchanges(ctx, queries.ListExchangesParams{
		UserID: userID, Limit: int64(page.Size()), Offset: int64(page.Offset),
	})
	if err != nil {
		return entity.List[entity.Conversion]{}, fmt.Errorf("ExchangeReadModel - History - r.q(ctx).ListExchanges: %w", err)
	}

	history := make([]entity.Conversion, 0, len(rows))
	for _, row := range rows {
		history = append(history, entity.Conversion{
			UserID:   userID,
			From:     row.FromCurrency,
			To:       row.ToCurrency,
			Amount:   row.Amount,
			Rate:     row.Rate,
			Result:   row.Result,
			RateDate: row.RateDate,
		})
	}

	return entity.List[entity.Conversion]{Items: history, Total: int(total)}, nil
}
