package user

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type loginInput struct {
	Body Login
}

type loginOutput struct {
	Body response.Token
}

func (r *controller) login(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "login",
		Summary:     "Login",
		Description: "Authenticate user and get JWT token",
		Method:      http.MethodPost,
		Path:        "/v1/auth/login",
		Tags:        []string{apidoc.AuthTag},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized, http.StatusTooManyRequests),
	}, func(ctx context.Context, input *loginInput) (*loginOutput, error) {
		signedIn, err := cqrs.SendQuery(ctx, r.qs, query.LogIn{Email: input.Body.Email, Password: input.Body.Password})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "login")
		}

		return &loginOutput{Body: response.Token{Token: signedIn.Token}}, nil
	})
}
