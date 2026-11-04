// Package user serves the v1 sign-up, password sign-in and profile operations over REST.
package user

import (
	"errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

const _usersTag = "users"

type controller struct {
	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    logger.Interface
}

// NewRoutes registers the user operations.
func NewRoutes(api huma.API, b cqrs.Buses, l logger.Interface) {
	r := &controller{cmds: b.Commands, qs: b.Queries, l: l}

	r.register(api)
	r.login(api)
	r.profile(api)
}

// problem maps a failed command or query to its REST problem: first what any command can fail
// with, then the user errors, and anything else to a logged 500.
func (r *controller) problem(err error, op string) error {
	if shared := apierr.Shared(err); shared != nil {
		r.l.Warn("restapi - v1 - %s: %v", op, err)

		return shared
	}

	switch {
	case errors.Is(err, entity.ErrUserAlreadyExists):
		return huma.Error409Conflict("user already exists")
	case errors.Is(err, entity.ErrInvalidCredentials):
		return huma.Error401Unauthorized("invalid credentials")
	case errors.Is(err, entity.ErrUserNotFound):
		return huma.Error404NotFound("user not found")
	default:
		r.l.Error(err, "restapi - v1 - "+op)

		return huma.Error500InternalServerError("internal server error")
	}
}
