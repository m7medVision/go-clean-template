package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/stretchr/testify/require"
)

const (
	owner  = "user-1"
	other  = "user-2"
	taskID = "task-1"
)

func createdAt() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC) }

// now is the tests' clock: an hour after the task was created, with nanoseconds the handlers drop.
func now() time.Time { return createdAt().Add(time.Hour + 999) }

func nowStored() time.Time { return createdAt().Add(time.Hour) }

// givenTask is a store holding owner's task, written at createdAt.
func givenTask(t *testing.T) *memstore.Store {
	t.Helper()

	store := memstore.New(entity.EventTypes())
	_, err := store.Save(context.Background(), entity.NewTaskAggregate(taskID, owner, "Write", "the spec", createdAt()))
	require.NoError(t, err)

	return store
}

// appended is what the store holds for the task after the first event.
func appended(store *memstore.Store) []entity.Event {
	return store.Events(entity.TaskStreamID(taskID))[1:]
}
