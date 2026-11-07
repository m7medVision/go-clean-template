package passkey

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (r *controller) beginPasskeyLogin(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "begin-passkey-login",
		Summary:     "Begin passkey login",
		Description: "Start a WebAuthn login ceremony for any registered passkey. Pass `options` to " +
			"navigator.credentials.get and send the result to finish with `ceremony_token` in the " +
			"X-Ceremony-Token header.",
		Method: http.MethodPost,
		Path:   "/v1/auth/passkey/begin",
		Tags:   []string{apidoc.AuthTag},
		Errors: apierr.QueryErrors(),
	}, func(ctx context.Context, _ *struct{}) (*ceremonyOutput, error) {
		started, err := cqrs.SendQuery(ctx, r.qs, query.BeginPasskeyLogin{})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "beginPasskeyLogin")
		}

		return &ceremonyOutput{Body: PasskeyCeremony{
			Options:       started.Options,
			CeremonyToken: started.Token,
		}}, nil
	})
}
