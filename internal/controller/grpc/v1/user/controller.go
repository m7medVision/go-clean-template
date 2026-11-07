// Package user serves AuthService (docs/proto/v1/auth.proto) over gRPC.
package user

import (
	"log/slog"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type Controller struct {
	v1.UnimplementedAuthServiceServer

	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

func New(b cqrs.Buses, l *slog.Logger) v1.AuthServiceServer {
	return &Controller{cmds: b.Commands, qs: b.Queries, l: l}
}

// PublicMethods are the AuthService methods a caller reaches without a token: signing up and
// signing in. Every other method of every service needs one.
func PublicMethods() []string {
	return []string{v1.AuthService_Register_FullMethodName, v1.AuthService_Login_FullMethodName}
}
