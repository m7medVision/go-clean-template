package event

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// CaptureUser sends the user domain's analytics events. It starts at the store's head, so the
// users registered before analytics was configured are never sent.
type CaptureUser struct {
	Analytics repo.Analytics
}

var (
	_ repo.Subscriber   = CaptureUser{}
	_ repo.StartsAtHead = CaptureUser{}
)

func (CaptureUser) Name() string { return "capture-user" }

func (CaptureUser) StartsAtHead() bool { return true }

func (CaptureUser) Handles() []entity.Event {
	return []entity.Event{entity.UserRegistered{}, entity.UserRegisteredViaOIDC{}}
}

func (h CaptureUser) Handle(ctx context.Context, e *entity.RecordedEvent) error {
	var (
		userID string
		method analytics.SignUpMethod
	)

	switch ev := e.Event.(type) {
	case entity.UserRegistered:
		userID, method = ev.UserID, analytics.SignUpPassword
	case entity.UserRegisteredViaOIDC:
		userID, method = ev.UserID, analytics.SignUpOIDC
	default:
		return fmt.Errorf("CaptureUser - %s: %w", e.Event.EventType(), entity.ErrUnhandledEvent)
	}

	err := h.Analytics.Capture(ctx, &analytics.Event{
		ID:         analytics.EventID(e),
		DistinctID: userID,
		At:         e.RecordedAt,
		Properties: analytics.UserRegistered{Method: method},
	})
	if err != nil {
		return fmt.Errorf("CaptureUser - h.Analytics.Capture: %w", err)
	}

	return nil
}
