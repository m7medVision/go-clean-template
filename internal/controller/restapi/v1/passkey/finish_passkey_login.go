package passkey

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type finishPasskeyLoginInput struct {
	CeremonyToken string `header:"X-Ceremony-Token" required:"true" minLength:"1" doc:"The ceremony_token from begin"`

	Body PasskeyCredential
}

type finishPasskeyLoginOutput struct {
	Body response.Token
}

func (r *controller) finishPasskeyLogin(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "finish-passkey-login",
		Summary:     "Finish passkey login",
		Description: "Verify the browser's assertion against the ceremony and get a JWT token. " +
			"A ceremony completes once; finishing it again is a conflict.",
		Method: http.MethodPost,
		Path:   "/v1/auth/passkey/finish",
		Tags:   []string{apidoc.AuthTag},
		Errors: apierr.CommandErrors(http.StatusBadRequest, http.StatusUnauthorized, http.StatusConflict),
	}, func(ctx context.Context, input *finishPasskeyLoginInput) (*finishPasskeyLoginOutput, error) {
		signedIn, err := cqrs.SendCommand(ctx, r.cmds,
			command.FinishPasskeyLogin{Token: input.CeremonyToken, Body: input.Body})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "finishPasskeyLogin")
		}

		return &finishPasskeyLoginOutput{Body: response.Token{Token: signedIn.Token}}, nil
	})
}
