package user

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (c *Controller) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginResponse, error) {
	signedIn, err := cqrs.SendQuery(ctx, c.qs, query.LogIn{Email: req.GetEmail(), Password: req.GetPassword()})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "Login")
	}

	return &v1.LoginResponse{Token: signedIn.Token}, nil
}
