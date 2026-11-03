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

func TestUpdateTaskRecordsTaskUpdated(t *testing.T) {
	t.Parallel()

	store := givenTask(t)
	h := command.UpdateTaskHandler{Store: store, Now: now}

	written, err := h.Handle(context.Background(),
		command.UpdateTask{UserID: owner, TaskID: taskID, Title: "Edit", Description: "the plan"})

	require.NoError(t, err)
	assert.Equal(t, taskID, written.ID)
	assert.Equal(t, []entity.Event{
		entity.TaskUpdated{TaskID: taskID, Title: "Edit", Description: "the plan", At: nowStored()},
	}, appended(store))
}

func TestUpdateTaskOfAMissingTaskIsNotFound(t *testing.T) {
	t.Parallel()

	h := command.UpdateTaskHandler{Store: memstore.New(entity.EventTypes())}

	_, err := h.Handle(context.Background(), command.UpdateTask{UserID: owner, TaskID: taskID, Title: "x"})

	require.ErrorIs(t, err, entity.ErrTaskNotFound)
}

func TestUpdateTaskOfAnotherUsersTaskIsForbidden(t *testing.T) {
	t.Parallel()

	store := givenTask(t)
	h := command.UpdateTaskHandler{Store: store}

	_, err := h.Handle(context.Background(), command.UpdateTask{UserID: other, TaskID: taskID, Title: "x"})

	require.ErrorIs(t, err, entity.ErrTaskForbidden)
	assert.Empty(t, appended(store))
}
