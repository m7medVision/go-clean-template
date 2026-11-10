package flags

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/flags/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type getFlagsOutput struct {
	Body entity.FlagValues
}

func (r *controller) getFlags(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-flags",
		Summary:     "Show feature flags",
		Description: "Show every feature flag as the current user sees it, with the values the backend uses",
		Method:      http.MethodGet,
		Path:        "/v1/flags",
		Tags:        []string{_tag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized),
	}, func(ctx context.Context, _ *struct{}) (*getFlagsOutput, error) {
		values, err := cqrs.SendQuery(ctx, r.qs, query.GetFlags{UserID: authn.MustCaller(ctx).UserID})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "getFlags")
		}

		return &getFlagsOutput{Body: values}, nil
	})
}
