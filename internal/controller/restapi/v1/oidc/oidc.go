package oidc

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/m7medvision/go-clean-template/internal/entity"
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
		enabled, err := cqrs.SendQuery[bool](ctx, r.qs, query.GetOIDCStatus{})
		if err != nil {
			return nil, r.oidcError(err, "oidcStatus")
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
		Errors: []int{http.StatusNotFound, http.StatusBadGateway, http.StatusInternalServerError},
	}, func(ctx context.Context, _ *struct{}) (*beginOIDCOutput, error) {
		started, err := cqrs.SendQuery[query.Started](ctx, r.qs, query.BeginOIDCLogin{})
		if err != nil {
			return nil, r.oidcError(err, "beginOIDC")
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
		Errors: []int{
			http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict,
			http.StatusBadGateway, http.StatusInternalServerError,
		},
	}, func(ctx context.Context, input *finishOIDCInput) (*finishOIDCOutput, error) {
		signedIn, err := cqrs.SendCommand[command.SignedIn](ctx, r.cmds, command.FinishOIDCLogin{
			Code: input.Body.Code, State: input.Body.State, Token: input.CeremonyToken,
		})
		if err != nil {
			return nil, r.oidcError(err, "finishOIDC")
		}

		return &finishOIDCOutput{Body: response.Token{Token: signedIn.Token}}, nil
	})
}

func (r *controller) oidcError(err error, op string) error {
	if shared := apierr.Shared(err); shared != nil {
		r.l.Warn("restapi - v1 - "+op, "error", err)

		return shared
	}

	switch {
	case errors.Is(err, entity.ErrOIDCDisabled):
		return huma.Error404NotFound("sso sign-in is not configured")
	case errors.Is(err, entity.ErrOIDCCeremonyInvalid):
		return huma.Error401Unauthorized("invalid sso sign-in")
	case errors.Is(err, entity.ErrAccountConflict):
		return huma.Error409Conflict("an account with this email or username already exists; sign in with it instead")
	case errors.Is(err, entity.ErrOIDCUnavailable):
		r.l.Error("restapi - v1 - "+op, "error", err)

		return huma.Error502BadGateway("sso provider unavailable")
	default:
		r.l.Error("restapi - v1 - "+op, "error", err)

		return huma.Error500InternalServerError("internal server error")
	}
}
