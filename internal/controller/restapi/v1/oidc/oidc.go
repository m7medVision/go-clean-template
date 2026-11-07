package oidc

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type oidcStatusOutput struct {
	Body OIDCStatus
}

type beginOIDCOutput struct {
	Body OIDCRedirect
}

type finishOIDCInput struct {
	CeremonyToken string `header:"X-Ceremony-Token" required:"true" minLength:"1" doc:"The ceremony_token from begin"`

	Body OIDCCallback
}

type finishOIDCOutput struct {
	Body response.Token
}

func (r *controller) oidcStatus(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "oidc-status",
		Summary:     "SSO status",
		Description: "Whether SSO sign-in through an OpenID Connect provider is configured.",
		Method:      http.MethodGet,
		Path:        "/v1/auth/oidc",
		Tags:        []string{apidoc.AuthTag},
	}, func(ctx context.Context, _ *struct{}) (*oidcStatusOutput, error) {
		enabled, err := cqrs.SendQuery(ctx, r.qs, query.GetOIDCStatus{})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "oidcStatus")
		}

		return &oidcStatusOutput{Body: OIDCStatus{Enabled: enabled}}, nil
	})
}

func (r *controller) beginOIDC(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "begin-oidc",
		Summary:     "Begin SSO sign-in",
		Description: "Start an SSO sign-in. Keep `ceremony_token` for this tab, send the browser to " +
			"`url`, and finish with the code and state the provider sends back to /sso/callback.",
		Method: http.MethodPost,
		Path:   "/v1/auth/oidc/begin",
		Tags:   []string{apidoc.AuthTag},
		Errors: apierr.QueryErrors(http.StatusNotFound, http.StatusBadGateway),
	}, func(ctx context.Context, _ *struct{}) (*beginOIDCOutput, error) {
		started, err := cqrs.SendQuery(ctx, r.qs, query.BeginOIDCLogin{})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "beginOIDC")
		}

		return &beginOIDCOutput{Body: OIDCRedirect{URL: started.AuthURL, CeremonyToken: started.Token}}, nil
	})
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
