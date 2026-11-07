package user

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
)

// Imports is what the importer returns for the rows q holds.
func Imports(ctx context.Context, q queries.Querier) ([]eventstore.Import, error) {
	return imports(ctx, q)
}

// Project is what the projector writes through q for e.
func Project(ctx context.Context, q queries.Querier, e *entity.RecordedEvent) error {
	return project(ctx, q, e)
}
