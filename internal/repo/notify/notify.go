// Package notify tells users about events on their account by enqueueing emails for the worker.
package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// Queue enqueues a message body on a named queue. The Postgres queue (internal/repo/persistent/queue)
// implements it, joining the transaction the context carries: an event handler's email commits with
// its checkpoint.
type Queue interface {
	Send(ctx context.Context, queue string, body json.RawMessage) (int64, error)
}

// Notifier enqueues account emails on the mail queue.
type Notifier struct {
	queue    Queue
	loginURL string
}

var _ repo.UserNotifier = (*Notifier)(nil)

// New returns a UserNotifier. loginURL is the sign-in page the welcome email links to.
func New(q Queue, loginURL string) repo.UserNotifier {
	return &Notifier{queue: q, loginURL: loginURL}
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
