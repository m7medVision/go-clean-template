// Package notify tells users about events on their account by publishing emails for the worker.
package notify

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// Publisher publishes a message body on a topic; *broker.Broker implements it. A second publish
// with the same id inside the broker's duplicate window is dropped, so an event handler that runs
// again after a crash does not send the email twice.
type Publisher interface {
	Publish(ctx context.Context, topic, id string, body []byte) error
}

// Notifier publishes account emails on the mail topic.
type Notifier struct {
	pub      Publisher
	loginURL string
}

var _ repo.UserNotifier = (*Notifier)(nil)

// New returns a UserNotifier. loginURL is the sign-in page the welcome email links to.
func New(p Publisher, loginURL string) repo.UserNotifier {
	return &Notifier{pub: p, loginURL: loginURL}
}

// Welcome publishes the welcome email; the worker sends it.
func (n *Notifier) Welcome(ctx context.Context, user *entity.User) error {
	welcome, err := entity.EncodeEmail(entity.WelcomeEmail{
		To:       user.Email,
		Username: user.Username,
		LoginURL: n.loginURL,
	})
	if err != nil {
		return fmt.Errorf("Notifier - Welcome - entity.EncodeEmail: %w", err)
	}

	if err = n.pub.Publish(ctx, entity.EmailQueue, "welcome-"+user.ID, welcome); err != nil {
		return fmt.Errorf("Notifier - Welcome - n.pub.Publish: %w", err)
	}

	return nil
}
