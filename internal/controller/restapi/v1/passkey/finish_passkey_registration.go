package passkey

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type finishPasskeyRegistrationInput struct {
	CeremonyToken string `header:"X-Ceremony-Token" required:"true" minLength:"1" doc:"The ceremony_token from begin"`
	Name          string `query:"name" maxLength:"64" doc:"A label for the passkey; defaults to \"Passkey\""`

	Body PasskeyCredential
}

func (r *controller) finishPasskeyRegistration(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "finish-passkey-registration",
		Summary:       "Finish adding a passkey",
		Description:   "Verify the browser's new credential against the ceremony and store it as a passkey.",
		Method:        http.MethodPost,
		Path:          "/v1/passkeys/register/finish",
		Tags:          []string{_tag},
		DefaultStatus: http.StatusCreated,
		Security:      []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:        apierr.CommandErrors(http.StatusBadRequest, http.StatusUnauthorized, http.StatusConflict),
	}, func(ctx context.Context, input *finishPasskeyRegistrationInput) (*passkeyOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		written, err := cqrs.SendCommand(ctx, r.cmds, command.FinishPasskeyRegistration{
			UserID: userID, Token: input.CeremonyToken, Name: input.Name, Body: input.Body,
		})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "finishPasskeyRegistration")
		}

		passkey, err := cqrs.SendQuery(ctx, r.qs, query.GetPasskey{UserID: userID, ID: written.ID})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "finishPasskeyRegistration")
		}

		return &passkeyOutput{Body: passkey}, nil
	})
}
