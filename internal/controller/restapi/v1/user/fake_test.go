package user_test

import (
	"context"
	"net/http"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// fakeUser answers the user command and queries on real buses.
type fakeUser struct {
	registered  entity.User
	registerErr error
	token       string
	loginErr    error
	getErr      error
}

func (f *fakeUser) buses() cqrs.Buses {
	b := resttest.NewBuses()

	cqrs.HandleCommand(b.Commands, cqrs.HandlerFunc[command.RegisterUser, cqrs.Written](
		func(context.Context, command.RegisterUser) (cqrs.Written, error) {
			return cqrs.Written{ID: f.registered.ID}, f.registerErr
		}))
	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[query.LogIn, string](
		func(context.Context, query.LogIn) (string, error) { return f.token, f.loginErr }))
	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[query.GetUser, entity.User](
		func(context.Context, query.GetUser) (entity.User, error) { return f.registered, f.getErr }))

	return b
}

// newRouter serves the REST router over u's buses; the use cases a user test never reaches are nil.
func newRouter(cfg *config.Config, u *fakeUser) http.Handler {
	return resttest.NewRouter(cfg, u.buses(), nil)
}
