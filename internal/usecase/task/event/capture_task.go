// Package event holds the task domain's event handlers, one file each: what follows from a task
// event, outside the task's own state.
package event

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// CaptureTask sends the task domain's analytics events. It starts at the store's head, so the
// tasks written before analytics was configured are never sent.
type CaptureTask struct {
	Analytics repo.Analytics
}

var (
	_ repo.Subscriber   = CaptureTask{}
	_ repo.StartsAtHead = CaptureTask{}
)

func (CaptureTask) Name() string { return "capture-task" }

func (CaptureTask) StartsAtHead() bool { return true }

func (CaptureTask) Handles() []entity.Event {
	return []entity.Event{entity.TaskCreated{}, entity.TaskTransitioned{}}
}

func (h CaptureTask) Handle(ctx context.Context, e *entity.RecordedEvent) error {
	var (
		userID string
		props  analytics.Properties
	)

	switch ev := e.Event.(type) {
	case entity.TaskCreated:
		userID, props = ev.UserID, analytics.TaskCreated{}
	case entity.TaskTransitioned:
		userID, props = e.Metadata.UserID, analytics.TaskStatusChanged{From: ev.From, To: ev.To}
	default:
		return fmt.Errorf("CaptureTask - %s: %w", e.Event.EventType(), entity.ErrUnhandledEvent)
	}

	err := h.Analytics.Capture(ctx, &analytics.Event{
		ID: analytics.EventID(e), DistinctID: userID, At: e.RecordedAt, Properties: props,
	})
	if err != nil {
		return fmt.Errorf("CaptureTask - h.Analytics.Capture: %w", err)
	}

	return nil
}
