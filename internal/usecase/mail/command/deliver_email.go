// Package command holds the mail domain's command, with its handler.
package command

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// DeliverEmail renders Email and sends it. Only the worker sends it, for each message on
// entity.EmailQueue. Mail holds no state, so there is no aggregate and nothing is stored: the
// delivery is the side effect. A render error will not go away on a retry; the caller tells it apart
// with errors.Is(err, entity.ErrUnknownEmail) for an email no template exists for.
type DeliverEmail struct {
	cqrs.Command[cqrs.Written]

	Email entity.Email
}

// DeliverEmailHandler handles DeliverEmail.
type DeliverEmailHandler struct {
	Renderer repo.MailRenderer
	Mailer   repo.Mailer
}

// Handle -.
func (h DeliverEmailHandler) Handle(ctx context.Context, cmd DeliverEmail) (cqrs.Written, error) {
	msg, err := h.Renderer.Render(ctx, cmd.Email)
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("DeliverEmail - h.Renderer.Render: %w", err)
	}

	if err = h.Mailer.Send(ctx, msg); err != nil {
		return cqrs.Written{}, fmt.Errorf("DeliverEmail - h.Mailer.Send: %w", err)
	}

	return cqrs.Written{}, nil
}
