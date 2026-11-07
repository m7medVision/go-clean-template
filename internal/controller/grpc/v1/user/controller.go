// Package user serves AuthService (docs/proto/v1/auth.proto) over gRPC.
package user

import (
	"errors"
	"log/slog"

	"github.com/go-playground/validator/v10"
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Controller -.
type Controller struct {
	v1.UnimplementedAuthServiceServer

	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
	v    *validator.Validate
}

// New -.
func New(b cqrs.Buses, l *slog.Logger) v1.AuthServiceServer {
	return &Controller{cmds: b.Commands, qs: b.Queries, l: l, v: validator.New(validator.WithRequiredStructEnabled())}
}

// fail maps a failed command or query to its status: first what any command can fail with, then
// the user errors, and anything else to a logged Internal.
func (c *Controller) fail(err error, op string) error {
	if shared := rpcerr.Shared(err); shared != nil {
		c.l.Warn("grpc - v1 - "+op, "error", err)

		return shared
	}

	switch {
	case errors.Is(err, entity.ErrUserAlreadyExists):
		return status.Error(codes.AlreadyExists, "user already exists")
	case errors.Is(err, entity.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, "invalid credentials")
	case errors.Is(err, entity.ErrUserNotFound):
		return status.Error(codes.NotFound, "user not found")
	default:
		c.l.Error("grpc - v1 - "+op, "error", err)

		return status.Error(codes.Internal, "internal server error")
	}
}
