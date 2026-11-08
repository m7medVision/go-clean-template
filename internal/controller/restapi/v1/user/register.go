package user

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type registerInput struct {
	Body Register
}

func (r *controller) register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "register",
		Summary:       "Register",
		Description:   "Register a new user",
		Method:        http.MethodPost,
		Path:          "/v1/auth/register",
		Tags:          []string{apidoc.AuthTag},
		DefaultStatus: http.StatusCreated,
		Errors:        apierr.CommandErrors(http.StatusConflict, http.StatusTooManyRequests),
	}, func(ctx context.Context, input *registerInput) (*userOutput, error) {
		written, err := cqrs.SendCommand(ctx, r.cmds, command.RegisterUser{
			Username: input.Body.Username, Email: input.Body.Email, Password: input.Body.Password,
		})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "register")
		}

		user, err := cqrs.SendQuery(ctx, r.qs, query.GetUser{UserID: written.ID})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "register")
		}

		return &userOutput{Body: user}, nil
	})
}
