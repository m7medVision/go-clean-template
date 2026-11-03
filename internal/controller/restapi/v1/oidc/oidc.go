package oidc

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/m7medvision/go-clean-template/internal/entity"
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
	}, func(_ context.Context, _ *struct{}) (*oidcStatusOutput, error) {
		return &oidcStatusOutput{Body: OIDCStatus{Enabled: r.oc.Enabled()}}, nil
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
		authURL, token, err := r.oc.Begin(ctx)
		if err != nil {
			return nil, r.oidcError(err, "beginOIDC")
		}

		return &beginOIDCOutput{Body: OIDCRedirect{URL: authURL, CeremonyToken: token}}, nil
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
		token, err := r.oc.Finish(ctx, input.Body.Code, input.Body.State, input.CeremonyToken)
		if err != nil {
			return nil, r.oidcError(err, "finishOIDC")
		}

		return &finishOIDCOutput{Body: response.Token{Token: token}}, nil
	})
}

func (r *controller) oidcError(err error, op string) error {
	switch {
	case errors.Is(err, entity.ErrOIDCDisabled):
		return huma.Error404NotFound("sso sign-in is not configured")
	case errors.Is(err, entity.ErrOIDCCeremonyInvalid):
		return huma.Error401Unauthorized("invalid sso sign-in")
	case errors.Is(err, entity.ErrAccountConflict):
		return huma.Error409Conflict("an account with this email or username already exists; sign in with it instead")
	case errors.Is(err, entity.ErrOIDCUnavailable):
		r.l.Error(err, "restapi - v1 - "+op)

		return huma.Error502BadGateway("sso provider unavailable")
	default:
		r.l.Error(err, "restapi - v1 - "+op)

		return huma.Error500InternalServerError("internal server error")
	}
}
