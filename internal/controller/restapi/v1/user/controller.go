// Package user serves the v1 sign-up, password sign-in and profile operations over REST.
package user

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

const _usersTag = "users"

type controller struct {
	u usecase.User
	l logger.Interface
}

// NewRoutes registers the user operations.
func NewRoutes(api huma.API, u usecase.User, l logger.Interface) {
	r := &controller{u: u, l: l}

	r.register(api)
	r.login(api)
	r.profile(api)
}
