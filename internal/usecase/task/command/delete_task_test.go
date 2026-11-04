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

func TestDeleteTaskRecordsTaskDeleted(t *testing.T) {
	t.Parallel()

	store := givenTask(t)
	h := command.DeleteTaskHandler{Store: store, Now: now}

	written, err := h.Handle(context.Background(), command.DeleteTask{UserID: owner, TaskID: taskID})

	require.NoError(t, err)
	assert.Equal(t, taskID, written.ID)
	assert.Equal(t, []entity.Event{entity.TaskDeleted{TaskID: taskID, At: nowStored()}}, appended(store))
}

func TestADeletedTaskCannotBeChangedAgain(t *testing.T) {
	t.Parallel()

	store := givenTask(t)
	deleteTask := command.DeleteTaskHandler{Store: store}

	_, err := deleteTask.Handle(context.Background(), command.DeleteTask{UserID: owner, TaskID: taskID})
	require.NoError(t, err)

	_, err = deleteTask.Handle(context.Background(), command.DeleteTask{UserID: owner, TaskID: taskID})
	require.ErrorIs(t, err, entity.ErrTaskNotFound)

	_, err = command.UpdateTaskHandler{Store: store}.Handle(context.Background(),
		command.UpdateTask{UserID: owner, TaskID: taskID, Title: "x"})
	require.ErrorIs(t, err, entity.ErrTaskNotFound)
}

func TestDeleteTaskOfAnotherUsersTaskIsNotFound(t *testing.T) {
	t.Parallel()

	store := givenTask(t)

	_, err := command.DeleteTaskHandler{Store: store}.Handle(context.Background(),
		command.DeleteTask{UserID: other, TaskID: taskID})

	require.ErrorIs(t, err, entity.ErrTaskNotFound)
	assert.Empty(t, appended(store))
}

func TestDeleteTaskOfAMissingTaskIsNotFound(t *testing.T) {
	t.Parallel()

	_, err := command.DeleteTaskHandler{Store: memstore.New(entity.EventTypes())}.Handle(context.Background(),
		command.DeleteTask{UserID: owner, TaskID: taskID})

	require.ErrorIs(t, err, entity.ErrTaskNotFound)
}
