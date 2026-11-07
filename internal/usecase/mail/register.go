// Package mail delivers email: its one command, DeliverEmail, lives in command/.
package mail

import (
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/mail/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Deps is what the mail handler is built from.
type Deps struct {
	Renderer repo.MailRenderer
	Mailer   repo.Mailer
}

// Register registers the mail command handler; mail answers no queries.
func Register(cmds *cqrs.Commands, _ *cqrs.Queries, d Deps) {
	cqrs.HandleCommand(cmds, command.DeliverEmailHandler{Renderer: d.Renderer, Mailer: d.Mailer})
}
