package user

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (c *Controller) GetProfile(ctx context.Context, _ *v1.GetProfileRequest) (*v1.GetProfileResponse, error) {
	userID := authn.MustCaller(ctx).UserID

	user, err := cqrs.SendQuery(ctx, c.qs, query.GetUser{UserID: userID})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "GetProfile")
	}

	return NewUserResponse[v1.GetProfileResponse](&user), nil
}
