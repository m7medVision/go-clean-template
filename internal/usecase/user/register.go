package user

import (
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Deps is what the user handlers are built from.
type Deps struct {
	Store    repo.EventStore
	Users    repo.UserReadModel
	Sessions repo.SessionIssuer
}

// Register registers every user command and query handler, one line each.
func Register(cmds *cqrs.Commands, qs *cqrs.Queries, d Deps) {
	cqrs.HandleCommand(cmds, command.RegisterUserHandler{Store: d.Store})

	cqrs.HandleQuery(qs, query.LogInHandler{Users: d.Users, Sessions: d.Sessions})
	cqrs.HandleQuery(qs, query.GetUserHandler{Users: d.Users})
}
