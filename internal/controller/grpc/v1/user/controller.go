// Package user serves AuthService (docs/proto/v1/auth.proto) over gRPC.
package user

import (
	"log/slog"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Controller -.
type Controller struct {
	v1.UnimplementedAuthServiceServer

	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

// New -.
func New(b cqrs.Buses, l *slog.Logger) v1.AuthServiceServer {
	return &Controller{cmds: b.Commands, qs: b.Queries, l: l}
}
