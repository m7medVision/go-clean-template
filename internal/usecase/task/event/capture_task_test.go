package event_test

import (
	"errors"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errQueue = errors.New("queue down")

func TestCaptureTaskMapsEachEvent(t *testing.T) {
	t.Parallel()

	at := time.Unix(1, 0).UTC()

	tests := []struct {
		name  string
		event entity.Event
		want  analytics.Properties
	}{
		{
			name:  "created",
			event: entity.TaskCreated{TaskID: "t1", UserID: "u1", Title: "secret"},
			want:  analytics.TaskCreated{},
		},
		{
			name:  "transitioned",
			event: entity.TaskTransitioned{TaskID: "t1", From: entity.TaskStatusTodo, To: entity.TaskStatusDone},
			want:  analytics.TaskStatusChanged{From: entity.TaskStatusTodo, To: entity.TaskStatusDone},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := repomock.NewMockAnalytics(gomock.NewController(t))
			a.EXPECT().Capture(gomock.Any(), &analytics.Event{
				ID: "task-t1-2", DistinctID: "u1", At: at, Properties: tt.want,
			}).Return(nil)

			require.NoError(t, event.CaptureTask{Analytics: a}.Handle(t.Context(), &entity.RecordedEvent{
				StreamID: "task-t1", StreamVersion: 2, RecordedAt: at,
				Metadata: entity.Metadata{UserID: "u1"}, Event: tt.event,
			}))
		})
	}
}

func TestCaptureTaskStartsAtHeadAndSkipsImports(t *testing.T) {
	t.Parallel()

	h := event.CaptureTask{}
	assert.True(t, h.StartsAtHead())
	assert.NotContains(t, h.Handles(), entity.Event(entity.TaskImported{}))
}

func TestCaptureTaskReturnsAFailureForTheRunnerToRetry(t *testing.T) {
	t.Parallel()

	a := repomock.NewMockAnalytics(gomock.NewController(t))
	a.EXPECT().Capture(gomock.Any(), gomock.Any()).Return(errQueue)

	require.ErrorIs(t, event.CaptureTask{Analytics: a}.Handle(t.Context(),
		&entity.RecordedEvent{StreamID: "task-t1", Event: entity.TaskCreated{}}), errQueue)
}
