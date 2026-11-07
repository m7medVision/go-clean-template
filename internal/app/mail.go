package app

import (
	"fmt"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/mailtmpl"
	"github.com/m7medvision/go-clean-template/internal/usecase/mail"
	"github.com/m7medvision/go-clean-template/pkg/smtp"
)

// The SMTP client is the Mailer the app wires in.
var _ repo.Mailer = (*smtp.Client)(nil)

// newMailDeps builds what DeliverEmail renders and sends with, through the relay m names. It fails
// when an embedded template is broken or MAIL_FROM is not one address, so either stops the binary
// at startup.
func newMailDeps(m *config.Mail) (mail.Deps, error) {
	renderer, err := mailtmpl.New()
	if err != nil {
		return mail.Deps{}, fmt.Errorf("mailtmpl.New: %w", err)
	}

	mailer, err := smtp.New(m.SMTPHost, m.SMTPPort, m.From, smtp.Auth(m.SMTPUsername, m.SMTPPassword))
	if err != nil {
		return mail.Deps{}, fmt.Errorf("smtp.New: %w", err)
	}

	return mail.Deps{Renderer: renderer, Mailer: mailer}, nil
}
