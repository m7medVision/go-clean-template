// Package mailer delivers rendered email over SMTP, through pkg/smtp.
package mailer

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/smtp"
)

// Sender is what the mailer sends through. *smtp.Client implements it.
type Sender interface {
	Send(ctx context.Context, m smtp.Message) error
}

// Mailer is the repo.Mailer on an SMTP client.
type Mailer struct {
	client Sender
}

var _ repo.Mailer = (*Mailer)(nil)

func New(client Sender) *Mailer {
	return &Mailer{client: client}
}

func (m *Mailer) Send(ctx context.Context, e repo.RenderedEmail) error {
	err := m.client.Send(ctx, smtp.Message{To: e.To, Subject: e.Subject, HTML: e.HTML, Text: e.Text})
	if err != nil {
		return fmt.Errorf("Mailer - Send: %w", err)
	}

	return nil
}
