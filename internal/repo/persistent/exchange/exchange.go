// Package exchange implements the Postgres-backed currency conversion history.
package exchange

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/repo/persistent/queries"
	"github.com/evrone/go-clean-template/pkg/postgres"
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

// GetHistory returns the user's most recent conversions, newest first.
func (r *Repo) GetHistory(ctx context.Context, userID string) ([]entity.Conversion, error) {
	rows, err := r.q.ListExchanges(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ExchangeRepo - GetHistory - r.q.ListExchanges: %w", err)
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

	return history, nil
}
