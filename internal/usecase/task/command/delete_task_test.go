package command_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errMongoDown = errors.New("mongo is down")

func TestDeleteTaskRecordsTaskDeletedAndDeletesItsAttributes(t *testing.T) {
	t.Parallel()

	for name, deleteErr := range map[string]error{"deleted": nil, "the store failing": errMongoDown} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := givenTask(t)
			attrs := repomock.NewMockTaskAttributesStore(gomock.NewController(t))
			attrs.EXPECT().Delete(gomock.Any(), taskID).Return(deleteErr)

			h := command.DeleteTaskHandler{Store: store, Attributes: attrs, Logger: slog.New(slog.DiscardHandler), Now: now}

			written, err := h.Handle(context.Background(), command.DeleteTask{UserID: owner, TaskID: taskID})

			require.NoError(t, err)
			assert.Equal(t, taskID, written.ID)
			assert.Equal(t, []entity.Event{entity.TaskDeleted{TaskID: taskID, At: now()}}, appended(store))
		})
	}
}

func TestDeleteTaskLeavesAttributesOfATaskItDidNotDelete(t *testing.T) {
	t.Parallel()

	attrs := repomock.NewMockTaskAttributesStore(gomock.NewController(t))
	h := command.DeleteTaskHandler{Store: givenTask(t), Attributes: attrs, Logger: slog.New(slog.DiscardHandler), Now: now}

	_, err := h.Handle(context.Background(), command.DeleteTask{UserID: "someone-else", TaskID: taskID})

	require.Error(t, err)
}
