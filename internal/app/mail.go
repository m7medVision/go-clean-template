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

// newMailDeps builds what DeliverEmail renders and sends with. It fails when an embedded template is
// broken, so a bad template stops the binary at startup.
func newMailDeps(cfg *config.Config) (mail.Deps, error) {
	renderer, err := mailtmpl.New()
	if err != nil {
		return mail.Deps{}, fmt.Errorf("mailtmpl.New: %w", err)
	}

	mailer := smtp.New(
		cfg.Mail.SMTPHost,
		cfg.Mail.SMTPPort,
		cfg.Mail.From,
		smtp.Auth(cfg.Mail.SMTPUsername, cfg.Mail.SMTPPassword),
	)

	return mail.Deps{Renderer: renderer, Mailer: mailer}, nil
}
