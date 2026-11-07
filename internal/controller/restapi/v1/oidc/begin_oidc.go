package oidc

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type beginOIDCOutput struct {
	Body OIDCRedirect
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
