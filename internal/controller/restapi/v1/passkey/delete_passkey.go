package passkey

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type passkeyPathInput struct {
	ID string `path:"id" example:"AQIDBA" doc:"The passkey's ID, unpadded base64url"`
}

type deletePasskeyOutput struct {
	Status int
}

func (r *controller) deletePasskey(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "delete-passkey",
		Summary:     "Delete a passkey",
		Description: "Delete one of the current user's passkeys; it can no longer sign in. " +
			"Another user's passkey is not found.",
		Method:        http.MethodDelete,
		Path:          "/v1/passkeys/{id}",
		Tags:          []string{_passkeysTag},
		Security:      []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		DefaultStatus: http.StatusNoContent,
		Errors:        apierr.CommandErrors(http.StatusUnauthorized, http.StatusNotFound),
	}, func(ctx context.Context, input *passkeyPathInput) (*deletePasskeyOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		err := r.delete(ctx, userID, input.ID)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "deletePasskey")
		}

		return &deletePasskeyOutput{Status: http.StatusNoContent}, nil
	})
}

func (r *controller) delete(ctx context.Context, userID, id string) error {
	_, err := cqrs.SendCommand(ctx, r.cmds, command.DeletePasskey{UserID: userID, ID: id})

	return err
}
