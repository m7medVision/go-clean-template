package user_test

import (
	"context"
	"net/http"

	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/user"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// fakeUser answers the user command and queries on real buses.
type fakeUser struct {
	registered entity.User
	token      string
}

func (f *fakeUser) buses() cqrs.Buses {
	b := resttest.NewBuses()

	cqrs.HandleCommand(b.Commands, cqrs.HandlerFunc[command.RegisterUser, entity.Written](
		func(context.Context, command.RegisterUser) (entity.Written, error) {
			return entity.Written{ID: f.registered.ID}, nil
		}))
	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[query.LogIn, entity.SignedIn](
		func(context.Context, query.LogIn) (entity.SignedIn, error) {
			return entity.SignedIn{Token: f.token}, nil
		}))
	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[query.GetUser, entity.User](
		func(context.Context, query.GetUser) (entity.User, error) { return f.registered, nil }))

	return b
}

// newRouter serves the REST router over u's buses; the use cases a user test never reaches are nil.
func newRouter(u *fakeUser) http.Handler {
	return resttest.NewRouter(u.buses(), user.NewRoutes)
}
