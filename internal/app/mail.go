package app

import (
	"fmt"

	workerv1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/mailtmpl"
	"github.com/m7medvision/go-clean-template/internal/usecase/mail"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/smtp"
	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// The SMTP client is the Mailer the worker wires in.
var _ repo.Mailer = (*smtp.Client)(nil)

// mailModule is email delivery, which only the worker runs: the mail queue and the DeliverEmail
// handler its messages send to, rendering with the embedded templates and sending through the relay
// the Mail settings name.
func mailModule() module {
	return module{
		queues: func(d *deps, b cqrs.Buses, w *worker.Worker) error {
			// A broken template or a MAIL_FROM that is not one address stops the worker at startup.
			renderer, err := mailtmpl.New()
			if err != nil {
				return fmt.Errorf("mail - mailtmpl.New: %w", err)
			}

			m := d.cfg.Mail

			mailer, err := smtp.New(m.SMTPHost, m.SMTPPort, m.From, smtp.Auth(m.SMTPUsername, m.SMTPPassword))
			if err != nil {
				return fmt.Errorf("mail - smtp.New: %w", err)
			}

			mail.Register(b.Commands, b.Queries, mail.Deps{Renderer: renderer, Mailer: mailer})
			workerv1.NewMailRoutes(w, b.Commands, d.l)

			return nil
		},
	}
}
