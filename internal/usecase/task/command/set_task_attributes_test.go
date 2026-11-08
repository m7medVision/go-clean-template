package command_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSetTaskAttributes(t *testing.T) {
	t.Parallel()

	patch := mongotask.AttributesPatch{Priority: mongotask.SetTo(mongotask.PriorityHigh)}

	tests := []struct {
		name    string
		task    entity.Task
		getErr  error
		patches bool
		want    error
	}{
		{"written", entity.Task{ID: taskID, UserID: owner}, nil, true, nil},
		{"not found", entity.Task{}, entity.ErrTaskNotFound, false, entity.ErrTaskNotFound},
		{"another user's task", entity.Task{ID: taskID, UserID: "someone-else"}, nil, false, entity.ErrTaskForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			tasks := repomock.NewMockTaskReadModel(ctrl)
			tasks.EXPECT().Get(gomock.Any(), taskID).Return(tt.task, tt.getErr)

			attrs := repomock.NewMockTaskAttributesStore(ctrl)
			if tt.patches {
				attrs.EXPECT().Patch(gomock.Any(), taskID, patch).Return(nil)
			}

			written, err := command.SetTaskAttributesHandler{Tasks: tasks, Attributes: attrs}.
				Handle(context.Background(), command.SetTaskAttributes{UserID: owner, TaskID: taskID, Patch: patch})

			if tt.want != nil {
				require.ErrorIs(t, err, tt.want)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, entity.Written{ID: taskID}, written)
		})
	}
}

func TestSetTaskAttributesValidatesThePatch(t *testing.T) {
	t.Parallel()

	cmd := command.SetTaskAttributes{Patch: mongotask.AttributesPatch{Estimate: mongotask.SetTo(0.1)}}
	assert.ErrorIs(t, cmd.Validate(), entity.ErrInvalidTaskEstimate)
}
