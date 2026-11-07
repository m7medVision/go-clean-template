// Package exchange implements the exchange read model on Postgres (exchange_history), the projector
// that writes it from the conversion events, and the importer that turns rows stored before
// conversions stored events into events.
package exchange

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Repo -.
type Repo struct {
	q *queries.Queries
}

// NewReadModel returns the exchange read model.
func NewReadModel(pg *postgres.Postgres) repo.ExchangeReadModel {
	return &Repo{queries.New(pg.Pool)}
}

// Conversion -.
func (r *Repo) Conversion(ctx context.Context, userID, conversionID string) (entity.Conversion, error) {
	row, err := r.q.GetConversion(ctx, queries.GetConversionParams{ConversionID: &conversionID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Conversion{}, entity.ErrConversionNotFound
		}

		return entity.Conversion{}, fmt.Errorf("ExchangeReadModel - Conversion - r.q.GetConversion: %w", err)
	}

	return entity.Conversion{
		From: row.FromCurrency, To: row.ToCurrency, Amount: row.Amount, Rate: row.Rate, Result: row.Result,
		RateDate: row.RateDate,
	}, nil
}

// History returns one page of the user's conversions, newest first, and their total count.
func (r *Repo) History(ctx context.Context, userID string, limit, offset int64) ([]entity.Conversion, int, error) {
	total, err := r.q.CountExchanges(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("ExchangeReadModel - History - r.q.CountExchanges: %w", err)
	}

	rows, err := r.q.ListExchanges(ctx, queries.ListExchangesParams{UserID: userID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("ExchangeReadModel - History - r.q.ListExchanges: %w", err)
	}

	history := make([]entity.Conversion, 0, len(rows))
	for _, row := range rows {
		history = append(history, entity.Conversion{
			From:     row.FromCurrency,
			To:       row.ToCurrency,
			Amount:   row.Amount,
			Rate:     row.Rate,
			Result:   row.Result,
			RateDate: row.RateDate,
		})
	}

	return history, int(total), nil
}
