package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
)

// Imports is what the importer returns for the rows q holds.
func Imports(ctx context.Context, q queries.Querier) ([]eventstore.Import, error) {
	return imports(ctx, q)
}
