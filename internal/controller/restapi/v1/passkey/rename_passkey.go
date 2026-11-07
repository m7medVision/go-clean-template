package passkey

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type renamePasskeyInput struct {
	ID   string `path:"id" example:"AQIDBA" doc:"The passkey's ID, unpadded base64url"`
	Body RenamePasskey
}

func (r *controller) renamePasskey(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "rename-passkey",
		Summary:     "Rename a passkey",
		Description: "Rename one of the current user's passkeys. Another user's passkey is not found.",
		Method:      http.MethodPatch,
		Path:        "/v1/passkeys/{id}",
		Tags:        []string{_tag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.CommandErrors(http.StatusUnauthorized, http.StatusNotFound),
	}, func(ctx context.Context, input *renamePasskeyInput) (*passkeyOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		passkey, err := r.rename(ctx, userID, input.ID, input.Body.Name)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "renamePasskey")
		}

		return &passkeyOutput{Body: passkey}, nil
	})
}

func (r *controller) rename(ctx context.Context, userID, id, name string) (entity.Passkey, error) {
	written, err := cqrs.SendCommand(ctx, r.cmds, command.RenamePasskey{UserID: userID, ID: id, Name: name})
	if err != nil {
		return entity.Passkey{}, err
	}

	return cqrs.SendQuery(ctx, r.qs, query.GetPasskey{UserID: userID, ID: written.ID})
}
