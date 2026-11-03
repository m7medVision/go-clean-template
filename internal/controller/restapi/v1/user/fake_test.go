package user_test

import (
	"context"
	"net/http"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
)

type fakeUser struct {
	usecase.User

	registered  entity.User
	registerErr error
	token       string
	loginErr    error
	getErr      error
}

func (f *fakeUser) Register(context.Context, string, string, string) (entity.User, error) {
	return f.registered, f.registerErr
}

func (f *fakeUser) Login(context.Context, string, string) (string, error) {
	return f.token, f.loginErr
}

func (f *fakeUser) GetUser(context.Context, string) (entity.User, error) {
	return f.registered, f.getErr
}

// newRouter serves the REST router over u; the use cases a user test never reaches are nil.
func newRouter(cfg *config.Config, u usecase.User) http.Handler {
	return resttest.NewRouter(cfg, nil, u, nil)
}
