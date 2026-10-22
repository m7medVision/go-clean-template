package app

import (
	"fmt"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/mailtmpl"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/internal/usecase/mail"
	"github.com/m7medvision/go-clean-template/pkg/smtp"
)

// The SMTP client is the Mailer the app wires in.
var _ repo.Mailer = (*smtp.Client)(nil)

// addMailUseCase adds the mail use case. It fails when an embedded template is broken.
func addMailUseCase(deps featureDeps, uc *useCases) error {
	mailUseCase, err := newMailUseCase(deps.cfg)
	if err != nil {
		return err
	}

	uc.mail = mailUseCase

	return nil
}

// newMailUseCase builds the mail use case the worker delivers with. It fails when an embedded
// template is broken, so a bad template stops the binary at startup.
func newMailUseCase(cfg *config.Config) (usecase.Mail, error) {
	renderer, err := mailtmpl.New()
	if err != nil {
		return nil, fmt.Errorf("mailtmpl.New: %w", err)
	}

	mailer := smtp.New(
		cfg.Mail.SMTPHost,
		cfg.Mail.SMTPPort,
		cfg.Mail.From,
		smtp.Auth(cfg.Mail.SMTPUsername, cfg.Mail.SMTPPassword),
	)

	return mail.New(renderer, mailer), nil
}
