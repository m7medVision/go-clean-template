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

type oidcStatusOutput struct {
	Body OIDCStatus
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
