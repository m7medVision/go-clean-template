package oidc

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type finishOIDCInput struct {
	CeremonyToken string `header:"X-Ceremony-Token" required:"true" minLength:"1" doc:"The ceremony_token from begin"`

	Body OIDCCallback
}

type finishOIDCOutput struct {
	Body response.Token
}

func (r *controller) finishOIDC(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "finish-oidc",
		Summary:     "Finish SSO sign-in",
		Description: "Exchange the provider's code for a JWT token. The first sign-in of an identity " +
			"creates its user; an email or username that already belongs to an account is a conflict.",
		Method: http.MethodPost,
		Path:   "/v1/auth/oidc/finish",
		Tags:   []string{apidoc.AuthTag},
		Errors: apierr.CommandErrors(http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict, http.StatusBadGateway),
	}, func(ctx context.Context, input *finishOIDCInput) (*finishOIDCOutput, error) {
		signedIn, err := cqrs.SendCommand(ctx, r.cmds, command.FinishOIDCLogin{
			Code: input.Body.Code, State: input.Body.State, Token: input.CeremonyToken,
		})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "finishOIDC")
		}

		return &finishOIDCOutput{Body: response.Token{Token: signedIn.Token}}, nil
	})
}
