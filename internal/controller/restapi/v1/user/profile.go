package user

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/usecase/user/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (r *controller) profile(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "profile",
		Summary:     "Get profile",
		Description: "Get current user profile",
		Method:      http.MethodGet,
		Path:        "/v1/user/profile",
		Tags:        []string{_tag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized, http.StatusNotFound),
	}, func(ctx context.Context, _ *struct{}) (*userOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		user, err := cqrs.SendQuery(ctx, r.qs, query.GetUser{UserID: userID})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "profile")
		}

		return &userOutput{Body: user}, nil
	})
}
