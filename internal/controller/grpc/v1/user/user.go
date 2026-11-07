package user

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	return NewRegisterResponse(&user), nil
}

// Login -.
func (c *Controller) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginResponse, error) {
	signedIn, err := cqrs.SendQuery(ctx, c.qs, query.LogIn{Email: req.GetEmail(), Password: req.GetPassword()})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "Login")
	}

	return &v1.LoginResponse{Token: signedIn.Token}, nil
}

// GetProfile -.
func (c *Controller) GetProfile(ctx context.Context, _ *v1.GetProfileRequest) (*v1.GetProfileResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	user, err := cqrs.SendQuery(ctx, c.qs, query.GetUser{UserID: userID})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "GetProfile")
	}

	return NewGetProfileResponse(&user), nil
}
