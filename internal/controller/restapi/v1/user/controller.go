// Package user serves the v1 sign-up, password sign-in and profile operations over REST.
package user

import (
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

const _usersTag = "users"

type controller struct {
	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

// NewRoutes registers the user operations.
func NewRoutes(api huma.API, b cqrs.Buses, l *slog.Logger) {
	r := &controller{cmds: b.Commands, qs: b.Queries, l: l}

	r.register(api)
	r.login(api)
	r.profile(api)
}
