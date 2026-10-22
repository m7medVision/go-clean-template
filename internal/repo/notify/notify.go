// Package notify tells users about events on their account by enqueueing emails for the worker.
package notify

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// Notifier enqueues account emails on the mail queue.
type Notifier struct {
	queue    repo.QueueRepo
	loginURL string
}

var _ repo.UserNotifier = (*Notifier)(nil)

// New returns a UserNotifier instrumented with OpenTelemetry tracing spans. loginURL is the sign-in
// page the welcome email links to.
func New(q repo.QueueRepo, loginURL string) repo.UserNotifier {
	return newTraced(&Notifier{queue: q, loginURL: loginURL})
}

// Welcome enqueues the welcome email; the worker sends it.
func (n *Notifier) Welcome(ctx context.Context, user *entity.User) error {
	welcome, err := entity.EncodeEmail(entity.WelcomeEmail{
		To:       user.Email,
		Username: user.Username,
		LoginURL: n.loginURL,
	})
	if err != nil {
		return fmt.Errorf("Notifier - Welcome - entity.EncodeEmail: %w", err)
	}

	_, err = n.queue.Send(ctx, entity.EmailQueue, welcome)
	if err != nil {
		return fmt.Errorf("Notifier - Welcome - n.queue.Send: %w", err)
	}

	return nil
}
