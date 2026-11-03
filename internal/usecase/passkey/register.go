package passkey

import (
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/ceremony"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

// Deps is what the passkey handlers are built from.
type Deps struct {
	Ceremony *ceremony.Ceremony
	Store    repo.EventStore
	Users    repo.UserReadModel
	Passkeys repo.PasskeyReadModel
	Guard    repo.ChallengeGuard
	Tx       repo.Transactor
	Tokens   *jwt.Manager
}

// Register registers every passkey command and query handler, one line each.
func Register(cmds *cqrs.Commands, qs *cqrs.Queries, d *Deps) {
	cqrs.HandleCommand(cmds, command.FinishPasskeyRegistrationHandler{Ceremony: d.Ceremony, Store: d.Store})
	cqrs.HandleCommand(cmds, command.RenamePasskeyHandler{Store: d.Store})
	cqrs.HandleCommand(cmds, command.DeletePasskeyHandler{Store: d.Store})
	cqrs.HandleCommand(cmds, &command.FinishPasskeyLoginHandler{
		Ceremony: d.Ceremony, Store: d.Store, Users: d.Users, Guard: d.Guard, Tx: d.Tx, Tokens: d.Tokens,
	})

	cqrs.HandleQuery(qs, query.BeginPasskeyRegistrationHandler{Ceremony: d.Ceremony, Users: d.Users, Passkeys: d.Passkeys})
	cqrs.HandleQuery(qs, query.BeginPasskeyLoginHandler{Ceremony: d.Ceremony})
	cqrs.HandleQuery(qs, query.ListPasskeysHandler{Passkeys: d.Passkeys})
	cqrs.HandleQuery(qs, query.GetPasskeyHandler{Passkeys: d.Passkeys})
}
