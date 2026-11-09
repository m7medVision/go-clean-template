package app

import (
	"fmt"

	workerv1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/internal/repo/mailer"
	"github.com/m7medvision/go-clean-template/internal/repo/mailtmpl"
	"github.com/m7medvision/go-clean-template/internal/usecase/mail"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/smtp"
)

// mailModule is email delivery, which only the worker runs: the mail topic and the DeliverEmail
// handler its messages send to, rendering with the embedded templates and sending through the relay
// the Mail settings name.
func mailModule() module {
	return module{
		queues: func(d *deps, b cqrs.Buses, r queueRouter) error {
			// A broken template or a MAIL_FROM that is not one address stops the worker at startup.
			renderer, err := mailtmpl.New()
			if err != nil {
				return fmt.Errorf("mail - mailtmpl.New: %w", err)
			}

			m := d.cfg.Mail

			client, err := smtp.New(m.SMTPHost, m.SMTPPort, m.From, smtp.Auth(m.SMTPUsername, m.SMTPPassword))
			if err != nil {
				return fmt.Errorf("mail - smtp.New: %w", err)
			}

			mail.Register(b.Commands, b.Queries, mail.Deps{Renderer: renderer, Mailer: mailer.New(client)})
			workerv1.NewMailRoutes(r, b.Commands, d.l)

			return nil
		},
	}
}
