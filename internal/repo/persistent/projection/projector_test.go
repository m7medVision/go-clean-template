package projection_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A projector hands each event to its project function, and names itself in what fails.
func TestAProjectorProjectsEachEventItIsHanded(t *testing.T) {
	t.Parallel()

	var projected []int64

	p := projection.NewProjector("ticks", []string{"ticks"}, []entity.Event{ticked{}}, &postgres.Postgres{},
		func(_ context.Context, _ queries.Querier, e *entity.RecordedEvent) error {
			projected = append(projected, e.Position)
			if e.Position == 2 {
				return errHandle
			}

			return nil
		})

	require.NoError(t, p.Handle(context.Background(), &entity.RecordedEvent{Position: 1, Event: ticked{N: 1}}))

	err := p.Handle(context.Background(), &entity.RecordedEvent{Position: 2, Event: ticked{N: 2}})
	require.ErrorIs(t, err, errHandle)
	assert.Contains(t, err.Error(), "ticks projector")

	assert.Equal(t, []int64{1, 2}, projected)
}
