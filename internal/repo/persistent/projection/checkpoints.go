package projection

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
)

// Checkpoints reads how far the projectors have got, for the app's wait step.
type Checkpoints struct {
	pg *postgres.Postgres
}

var _ repo.ProjectionCheckpoints = (*Checkpoints)(nil)

func NewCheckpoints(pg *postgres.Postgres) *Checkpoints {
	return &Checkpoints{pg: pg}
}

// Reached reports whether every projector in names has handled the event at position. A projector
// that has not started yet has no checkpoint, and so has not reached it.
func (c *Checkpoints) Reached(ctx context.Context, names []string, position int64) (bool, error) {
	n, err := queries.New(c.pg.Executor(ctx)).CheckpointsReached(ctx, queries.CheckpointsReachedParams{
		Names: names, MinPosition: position,
	})
	if err != nil {
		return false, fmt.Errorf("projection - Checkpoints - Reached: %w", err)
	}

	return int(n) == len(names), nil
}
