package repo

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

//go:generate mockgen -source=mail.go -destination=repomock/mail.go -package=repomock

// RenderedEmail is one email ready to send: whom to, its subject, and its HTML and text parts. The
// mailer sets the sender.
type RenderedEmail struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

type (
	// MailRenderer turns an email into the message the mailer sends, from the templates embedded in
	// the binary.
	MailRenderer interface {
		Render(ctx context.Context, email entity.Email) (RenderedEmail, error)
	}

	// Mailer delivers a rendered email. The SMTP mailer (internal/repo/mailer) implements it;
	// another provider is another implementation.
	Mailer interface {
		Send(ctx context.Context, e RenderedEmail) error
	}
)
