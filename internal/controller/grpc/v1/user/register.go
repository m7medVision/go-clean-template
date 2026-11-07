package user

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Register -.
func (c *Controller) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.RegisterResponse, error) {
	written, err := cqrs.SendCommand(ctx, c.cmds, command.RegisterUser{
		Username: req.GetUsername(), Email: req.GetEmail(), Password: req.GetPassword(),
	})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "Register")
	}

	user, err := cqrs.SendQuery(ctx, c.qs, query.GetUser{UserID: written.ID})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "Register")
	}

	return NewUserResponse[v1.RegisterResponse](&user), nil
}
