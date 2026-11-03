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

func TestTransitionTaskRecordsTaskTransitioned(t *testing.T) {
	t.Parallel()

	store := givenTask(t)
	h := command.TransitionTaskHandler{Store: store, Now: now}

	written, err := h.Handle(context.Background(),
		command.TransitionTask{UserID: owner, TaskID: taskID, Status: entity.TaskStatusInProgress})

	require.NoError(t, err)
	assert.Equal(t, taskID, written.ID)
	assert.Equal(t, []entity.Event{entity.TaskTransitioned{
		TaskID: taskID, From: entity.TaskStatusTodo, To: entity.TaskStatusInProgress, At: nowStored(),
	}}, appended(store))
}

func TestTransitionTaskRefusesAMoveTheStatusDoesNotAllow(t *testing.T) {
	t.Parallel()

	store := givenTask(t)
	h := command.TransitionTaskHandler{Store: store}

	_, err := h.Handle(context.Background(), command.TransitionTask{UserID: owner, TaskID: taskID, Status: entity.TaskStatusDone})

	require.ErrorIs(t, err, entity.ErrInvalidTransition)
	assert.Empty(t, appended(store))
}

func TestTransitionTaskOfAnotherUsersTaskIsForbidden(t *testing.T) {
	t.Parallel()

	h := command.TransitionTaskHandler{Store: givenTask(t)}

	_, err := h.Handle(context.Background(),
		command.TransitionTask{UserID: other, TaskID: taskID, Status: entity.TaskStatusInProgress})

	require.ErrorIs(t, err, entity.ErrTaskForbidden)
}

func TestTransitionTaskOfAMissingTaskIsNotFound(t *testing.T) {
	t.Parallel()

	h := command.TransitionTaskHandler{Store: memstore.New(entity.EventTypes())}

	_, err := h.Handle(context.Background(),
		command.TransitionTask{UserID: owner, TaskID: taskID, Status: entity.TaskStatusInProgress})

	require.ErrorIs(t, err, entity.ErrTaskNotFound)
}
