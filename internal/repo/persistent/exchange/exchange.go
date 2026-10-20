// Package exchange implements the Postgres-backed currency conversion history.
package exchange

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Repo -.
type Repo struct {
	q *queries.Queries
}

// New returns an Exchange repository instrumented with OpenTelemetry tracing spans.
func New(pg *postgres.Postgres) repo.ExchangeRepo {
	return newTraced(&Repo{queries.New(pg.Pool)})
}

// Store -.
func (r *Repo) Store(ctx context.Context, userID string, c *entity.Conversion) error {
	err := r.q.CreateExchange(ctx, queries.CreateExchangeParams{
		UserID:       userID,
		FromCurrency: c.From,
		ToCurrency:   c.To,
		Amount:       c.Amount,
		Rate:         c.Rate,
		Result:       c.Result,
		RateDate:     c.RateDate,
	})
	if err != nil {
		return fmt.Errorf("ExchangeRepo - Store - r.q.CreateExchange: %w", err)
	}

	return nil
}

// GetHistory returns one page of the user's conversions, newest first, and their total count.
func (r *Repo) GetHistory(ctx context.Context, userID string, limit, offset int64) ([]entity.Conversion, int, error) {
	total, err := r.q.CountExchanges(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("ExchangeRepo - GetHistory - r.q.CountExchanges: %w", err)
	}

	rows, err := r.q.ListExchanges(ctx, queries.ListExchangesParams{UserID: userID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("ExchangeRepo - GetHistory - r.q.ListExchanges: %w", err)
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
