package aggregate_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/m7medvision/go-clean-template/internal/usecase/aggregate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errRule = errors.New("rule broken")

func at() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC) }

func TestSaveSavesANewAggregate(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())
	task := entity.NewTaskAggregate("t1", "u1", "a", "b", at())

	written, err := aggregate.Save(context.Background(), store, task)

	require.NoError(t, err)
	assert.Equal(t, "t1", written.ID)
	assert.Positive(t, written.Position)
	assert.Equal(t, []string{"TaskCreated"}, written.Events, "the wait step waits on the projectors of these")
	assert.Len(t, store.Events(entity.TaskStreamID("t1")), 1)
}

func TestChangeLoadsChangesAndSaves(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())
	_, err := aggregate.Save(context.Background(), store, entity.NewTaskAggregate("t1", "u1", "a", "b", at()))
	require.NoError(t, err)

	written, err := aggregate.Change(context.Background(), store, "t1", func(t *entity.TaskAggregate) error { return t.Update("u1", "c", "d", at()) })

	require.NoError(t, err)
	assert.Equal(t, "t1", written.ID)
	assert.Len(t, store.Events(entity.TaskStreamID("t1")), 2)
}

func TestChangeOfAMissingStreamIsTheNotFoundError(t *testing.T) {
	t.Parallel()

	_, err := aggregate.Change(context.Background(), memstore.New(entity.EventTypes()), "t1", func(*entity.TaskAggregate) error { return nil })

	require.ErrorIs(t, err, entity.ErrTaskNotFound)
}

// A passkey ID that is not base64url names no stream: it is not found, without a load.
func TestLoadOfAnIDThatNamesNoStreamIsTheNotFoundError(t *testing.T) {
	t.Parallel()

	_, err := aggregate.Load[entity.PasskeyAggregate](context.Background(), memstore.New(entity.EventTypes()), "not base64!")

	require.ErrorIs(t, err, entity.ErrPasskeyNotFound)
}

func TestARefusedChangeSavesNothing(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())
	_, err := aggregate.Save(context.Background(), store, entity.NewTaskAggregate("t1", "u1", "a", "b", at()))
	require.NoError(t, err)

	_, err = aggregate.Change(context.Background(), store, "t1", func(*entity.TaskAggregate) error { return errRule })

	require.ErrorIs(t, err, errRule)
	assert.Len(t, store.Events(entity.TaskStreamID("t1")), 1)
}

func TestNowIsUTC(t *testing.T) {
	t.Parallel()

	local := time.Date(2026, 1, 1, 13, 0, 0, 123456789, time.FixedZone("GST", 4*60*60))

	assert.Equal(t, time.Date(2026, 1, 1, 9, 0, 0, 123456789, time.UTC), aggregate.Now(func() time.Time { return local }))
	assert.Equal(t, time.UTC, aggregate.Now(nil).Location())
}
