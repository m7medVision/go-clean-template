package passkey

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type beginPasskeyRegistrationInput struct{}

func (r *controller) beginPasskeyRegistration(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "begin-passkey-registration",
		Summary:     "Begin adding a passkey",
		Description: "Start a WebAuthn registration ceremony for the current user. Pass `options` to " +
			"navigator.credentials.create and send the result to finish with `ceremony_token` in the " +
			"X-Ceremony-Token header. The caller must have signed in recently (WEBAUTHN_REAUTH_MAX_AGE); " +
			"otherwise the answer is a 401 whose detail is \"" + entity.ErrPasskeyReauthRequired.Error() +
			"\": sign in again and retry.",
		Method:   http.MethodPost,
		Path:     "/v1/passkeys/register/begin",
		Tags:     []string{_tag},
		Security: []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:   apierr.QueryErrors(http.StatusUnauthorized, http.StatusNotFound),
	}, func(ctx context.Context, _ *beginPasskeyRegistrationInput) (*ceremonyOutput, error) {
		// IssuedAt is when the caller last signed in: the use case asks a stale one to sign in again.
		caller := authn.MustCaller(ctx)

		started, err := r.beginRegistration(ctx, caller.UserID, caller.IssuedAt)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "beginPasskeyRegistration")
		}

		return &ceremonyOutput{Body: PasskeyCeremony{
			Options:       started.Options,
			CeremonyToken: started.Token,
		}}, nil
	})
}

func (r *controller) beginRegistration(ctx context.Context, userID string, authTime time.Time) (entity.CeremonyStarted, error) {
	return cqrs.SendQuery(ctx, r.qs, query.BeginPasskeyRegistration{UserID: userID, AuthTime: authTime})
}
