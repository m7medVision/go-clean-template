package command_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTaskRecordsTaskCreated(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())
	h := command.CreateTaskHandler{Store: store, Now: now}

	written, err := h.Handle(context.Background(), command.CreateTask{UserID: owner, Title: "Write", Description: "the spec"})

	require.NoError(t, err)
	assert.NotEmpty(t, written.ID)
	assert.Positive(t, written.Position)
	assert.Equal(t, []entity.Event{entity.TaskCreated{
		TaskID: written.ID, UserID: owner, Title: "Write", Description: "the spec", At: nowStored(),
	}}, store.Events(entity.TaskStreamID(written.ID)))
}

func TestEachCreateTaskGetsItsOwnID(t *testing.T) {
	t.Parallel()

	h := command.CreateTaskHandler{Store: memstore.New(entity.EventTypes())}

	first, err := h.Handle(context.Background(), command.CreateTask{UserID: owner, Title: "a"})
	require.NoError(t, err)

	second, err := h.Handle(context.Background(), command.CreateTask{UserID: owner, Title: "a"})
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID)
}
