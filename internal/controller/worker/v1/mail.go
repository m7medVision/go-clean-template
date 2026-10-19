package v1

import (
	"context"
	"errors"
	"fmt"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/worker"
)

// Handle delivers one email from entity.EmailQueue. The body is the envelope entity.EncodeEmail
// wrote, decoded with entity.DecodeEmail rather than a request DTO: both ends are this codebase.
//
// Delivery is at-least-once. A worker that stops after the relay accepts the email but before the
// message is deleted sends it again, so the recipient can get a duplicate; for a welcome email that
// is accepted rather than tracked.
func (c *MailController) Handle(ctx context.Context, msg *worker.Message) error {
	email, err := entity.DecodeEmail(msg.Body)
	// A body that does not decode, or names an email this build does not know, never will, so it is
	// dropped rather than retried.
	if err != nil {
		c.l.Error(fmt.Errorf("worker - v1 - Mail - entity.DecodeEmail: message %d: %w", msg.ID, err))

		return nil
	}

	err = c.mail.Deliver(ctx, email)
	if errors.Is(err, entity.ErrUnknownEmail) {
		c.l.Error(fmt.Errorf("worker - v1 - Mail - c.mail.Deliver: message %d: %w", msg.ID, err))

		return nil
	}

	if err != nil {
		return fmt.Errorf("worker - v1 - Mail - c.mail.Deliver: message %d: %w", msg.ID, err)
	}

	return nil
}
