package passkey

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type listPasskeysOutput struct {
	Body PasskeyList
}

func (r *controller) listPasskeys(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-passkeys",
		Summary:     "List passkeys",
		Description: "List the current user's passkeys, oldest first",
		Method:      http.MethodGet,
		Path:        "/v1/passkeys",
		Tags:        []string{_tag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized),
	}, func(ctx context.Context, _ *struct{}) (*listPasskeysOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		passkeys, err := cqrs.SendQuery(ctx, r.qs, query.ListPasskeys{UserID: userID})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "listPasskeys")
		}

		return &listPasskeysOutput{Body: NewPasskeyList(passkeys)}, nil
	})
}
