// Package v1 handles the queue messages the worker consumes: each handler decodes a message, sends
// a command on the buses, and returns its error so the message is retried.
package v1

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/mail/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// MailController -.
type MailController struct {
	cmds *cqrs.Commands
	l    *slog.Logger
}

// NewMailController -.
func NewMailController(cmds *cqrs.Commands, l *slog.Logger) *MailController {
	return &MailController{cmds: cmds, l: l}
}

// NewMailRoutes registers the handler of entity.EmailQueue on w, delivering through cmds.
func NewMailRoutes(w *worker.Worker, cmds *cqrs.Commands, l *slog.Logger) {
	w.Handle(entity.EmailQueue, NewMailController(cmds, l).Handle)
}

// Handle delivers one email from entity.EmailQueue. The body is the envelope entity.EncodeEmail
// wrote, decoded with entity.DecodeEmail rather than a request DTO: both ends are this codebase.
//
// Delivery is at-least-once. A worker that stops after the relay accepts the email but before the
// message is deleted sends it again, so the recipient can get a duplicate; for transactional email that
// is accepted rather than tracked.
func (c *MailController) Handle(ctx context.Context, msg *worker.Message) error {
	email, err := entity.DecodeEmail(msg.Body)
	// A body that does not decode, or names an email this build does not know, never will, so it is
	// dropped rather than retried.
	if err != nil {
		c.l.Error("worker - v1 - mail - decode", "message", msg.ID, "error", err)

		return nil
	}

	_, err = cqrs.SendCommand(ctx, c.cmds, command.DeliverEmail{Email: email})
	if errors.Is(err, entity.ErrUnknownEmail) {
		c.l.Error("worker - v1 - mail - deliver", "message", msg.ID, "error", err)

		return nil
	}

	if err != nil {
		return fmt.Errorf("worker - v1 - Mail - DeliverEmail: message %d: %w", msg.ID, err)
	}

	return nil
}
