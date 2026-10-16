package persistent

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

// New returns a Translation repository instrumented with OpenTelemetry tracing spans.
func New(pg *postgres.Postgres) repo.TranslationRepo {
	return newTraced(&Repo{queries.New(pg.Pool)})
}

// GetHistory -.
func (r *Repo) GetHistory(ctx context.Context, userID string) ([]entity.Translation, error) {
	rows, err := r.q.ListHistory(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("TranslationRepo - GetHistory - r.q.ListHistory: %w", err)
	}

	entities := make([]entity.Translation, len(rows))
	for i, row := range rows {
		entities[i] = entity.Translation(row)
	}

	return entities, nil
}

// Store -.
func (r *Repo) Store(ctx context.Context, userID string, t entity.Translation) error {
	err := r.q.CreateHistory(ctx, queries.CreateHistoryParams{
		UserID:      userID,
		Source:      t.Source,
		Destination: t.Destination,
		Original:    t.Original,
		Translation: t.Translation,
	})
	if err != nil {
		return fmt.Errorf("TranslationRepo - Store - r.q.CreateHistory: %w", err)
	}

	return nil
}
