package app

import (
	"fmt"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/repo/mailer"
	"github.com/m7medvision/go-clean-template/internal/repo/mailtmpl"
	"github.com/m7medvision/go-clean-template/internal/usecase/mail"
	"github.com/m7medvision/go-clean-template/pkg/smtp"
)

// newMailDeps builds what DeliverEmail renders and sends with. It fails when an embedded template is
// broken, so a bad template stops the binary at startup.
func newMailDeps(cfg *config.Config) (mail.Deps, error) {
	renderer, err := mailtmpl.New()
	if err != nil {
		return mail.Deps{}, fmt.Errorf("mailtmpl.New: %w", err)
	}

	client := smtp.New(
		cfg.Mail.SMTPHost,
		cfg.Mail.SMTPPort,
		cfg.Mail.From,
		smtp.Auth(cfg.Mail.SMTPUsername, cfg.Mail.SMTPPassword),
	)

	return mail.Deps{Renderer: renderer, Mailer: mailer.New(client)}, nil
}
