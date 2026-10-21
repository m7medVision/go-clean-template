package v1

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

type oidcStatusOutput struct {
	Body response.OIDCStatus
}

type beginOIDCOutput struct {
	Body response.OIDCRedirect
}

type finishOIDCInput struct {
	CeremonyToken string `header:"X-Ceremony-Token" required:"true" minLength:"1" doc:"The ceremony_token from begin"`

	Body request.OIDCCallback
}

type finishOIDCOutput struct {
	Body response.Token
}

func (r *V1) oidcStatus(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "oidc-status",
		Summary:     "SSO status",
		Description: "Whether SSO sign-in through an OpenID Connect provider is configured.",
		Method:      http.MethodGet,
		Path:        "/v1/auth/oidc",
		Tags:        []string{_authTag},
	}, func(_ context.Context, _ *struct{}) (*oidcStatusOutput, error) {
		return &oidcStatusOutput{Body: response.OIDCStatus{Enabled: r.oc.Enabled()}}, nil
	})
}

func (r *V1) beginOIDC(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "begin-oidc",
		Summary:     "Begin SSO sign-in",
		Description: "Start an SSO sign-in. Keep `ceremony_token` for this tab, send the browser to " +
			"`url`, and finish with the code and state the provider sends back to /sso/callback.",
		Method: http.MethodPost,
		Path:   "/v1/auth/oidc/begin",
		Tags:   []string{_authTag},
		Errors: []int{http.StatusNotFound, http.StatusBadGateway, http.StatusInternalServerError},
	}, func(ctx context.Context, _ *struct{}) (*beginOIDCOutput, error) {
		authURL, token, err := r.oc.Begin(ctx)
		if err != nil {
			return nil, r.oidcError(err, "beginOIDC")
		}

		return &beginOIDCOutput{Body: response.OIDCRedirect{URL: authURL, CeremonyToken: token}}, nil
	})
}

func (r *V1) finishOIDC(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "finish-oidc",
		Summary:     "Finish SSO sign-in",
		Description: "Exchange the provider's code for a JWT token. The first sign-in of an identity " +
			"creates its user; an email or username that already belongs to an account is a conflict.",
		Method: http.MethodPost,
		Path:   "/v1/auth/oidc/finish",
		Tags:   []string{_authTag},
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

func (r *V1) oidcError(err error, op string) error {
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
