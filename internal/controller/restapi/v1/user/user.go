package user

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type registerInput struct {
	Body Register
}

// userOutput is the user an operation wrote or read.
type userOutput struct {
	Body entity.User
}

type loginInput struct {
	Body Login
}

type loginOutput struct {
	Body response.Token
}

type profileInput struct{}

func (r *controller) register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "register",
		Summary:       "Register",
		Description:   "Register a new user",
		Method:        http.MethodPost,
		Path:          "/v1/auth/register",
		Tags:          []string{apidoc.AuthTag},
		DefaultStatus: http.StatusCreated,
		Errors:        apierr.CommandErrors(http.StatusConflict),
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

func (r *controller) login(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "login",
		Summary:     "Login",
		Description: "Authenticate user and get JWT token",
		Method:      http.MethodPost,
		Path:        "/v1/auth/login",
		Tags:        []string{apidoc.AuthTag},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized),
	}, func(ctx context.Context, input *loginInput) (*loginOutput, error) {
		signedIn, err := cqrs.SendQuery(ctx, r.qs, query.LogIn{Email: input.Body.Email, Password: input.Body.Password})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "login")
		}

		return &loginOutput{Body: response.Token{Token: signedIn.Token}}, nil
	})
}

func (r *controller) profile(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "profile",
		Summary:     "Get profile",
		Description: "Get current user profile",
		Method:      http.MethodGet,
		Path:        "/v1/user/profile",
		Tags:        []string{_usersTag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized, http.StatusNotFound),
	}, func(ctx context.Context, _ *profileInput) (*userOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		user, err := cqrs.SendQuery(ctx, r.qs, query.GetUser{UserID: userID})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "profile")
		}

		return &userOutput{Body: user}, nil
	})
}
