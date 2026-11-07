package command_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
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
	assert.Equal(t, []entity.Event{entity.TaskDeleted{TaskID: taskID, At: now()}}, appended(store))
}
