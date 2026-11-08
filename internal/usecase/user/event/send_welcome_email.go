// Package event holds the user domain's event handlers, one file each: what follows from a user
// event, outside the user's own state.
package event

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// SendWelcomeEmail publishes the welcome email for each user who registers with a password. Users
// imported by the backfill, and users created by an SSO sign-in, get none. The worker runs it before
// it moves its checkpoint, so a crash in between publishes again; the message ID is the user's, so
// the broker drops the repeat and each user gets the email once.
type SendWelcomeEmail struct {
	Notifier repo.UserNotifier
}

var _ repo.Subscriber = SendWelcomeEmail{}

func (SendWelcomeEmail) Name() string {
	return "send-welcome-email"
}

func (SendWelcomeEmail) Handles() []entity.Event {
	return []entity.Event{entity.UserRegistered{}}
}

func (h SendWelcomeEmail) Handle(ctx context.Context, e *entity.RecordedEvent) error {
	registered, ok := e.Event.(entity.UserRegistered)
	if !ok {
		return fmt.Errorf("SendWelcomeEmail - %s: %w", e.Event.EventType(), entity.ErrUnhandledEvent)
	}

	user := entity.User{ID: registered.UserID, Username: registered.Username, Email: registered.Email}
	if err := h.Notifier.Welcome(ctx, &user); err != nil {
		return fmt.Errorf("SendWelcomeEmail - h.Notifier.Welcome: %w", err)
	}

	return nil
}
