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
	"github.com/m7medvision/go-clean-template/pkg/broker"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Router registers a handler for a topic; *broker.Broker is one.
type Router interface {
	Handle(topic string, h broker.HandlerFunc)
}

type MailController struct {
	cmds *cqrs.Commands
	l    *slog.Logger
}

func NewMailController(cmds *cqrs.Commands, l *slog.Logger) *MailController {
	return &MailController{cmds: cmds, l: l}
}

func NewMailRoutes(r Router, cmds *cqrs.Commands, l *slog.Logger) {
	r.Handle(entity.EmailQueue, NewMailController(cmds, l).Handle)
}

// Handle delivers one email from entity.EmailQueue. The body is the envelope entity.EncodeEmail
// wrote, decoded with entity.DecodeEmail rather than a request DTO: both ends are this codebase.
//
// Delivery is at-least-once. A worker that stops after the relay accepts the email but before the
// message is acknowledged sends it again, so the recipient can get a duplicate; for transactional
// email that is accepted rather than tracked.
func (c *MailController) Handle(ctx context.Context, body []byte) error {
	email, err := entity.DecodeEmail(body)
	// A body that does not decode, or names an email this build does not know, never will: it goes
	// to the poison topic without retries, kept rather than dropped.
	if err != nil {
		return broker.Permanent(fmt.Errorf("worker - v1 - mail - decode: %w", err))
	}

	_, err = cqrs.SendCommand(ctx, c.cmds, command.DeliverEmail{Email: email})
	if errors.Is(err, entity.ErrUnknownEmail) {
		return broker.Permanent(fmt.Errorf("worker - v1 - mail - deliver: %w", err))
	}

	if err != nil {
		return fmt.Errorf("worker - v1 - Mail - DeliverEmail: %w", err)
	}

	return nil
}
