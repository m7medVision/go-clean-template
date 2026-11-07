package passkey

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

type beginPasskeyRegistrationInput struct{}

// ceremonyOutput is a WebAuthn ceremony a begin operation started.
type ceremonyOutput struct {
	Body PasskeyCeremony
}

type finishPasskeyRegistrationInput struct {
	CeremonyToken string `header:"X-Ceremony-Token" required:"true" minLength:"1" doc:"The ceremony_token from begin"`
	Name          string `query:"name" maxLength:"64" doc:"A label for the passkey; defaults to \"Passkey\""`

	Body PasskeyCredential
}

// passkeyOutput is the passkey an operation wrote.
type passkeyOutput struct {
	Body entity.Passkey
}

type listPasskeysInput struct{}

type listPasskeysOutput struct {
	Body PasskeyList
}

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
		Tags:     []string{_passkeysTag},
		Security: []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:   apierr.QueryErrors(http.StatusUnauthorized, http.StatusNotFound),
	}, func(ctx context.Context, _ *beginPasskeyRegistrationInput) (*ceremonyOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		// A missing auth time is zero, which the use case treats as a sign-in too old to add a passkey.
		authTime, _ := middleware.AuthTime(ctx)

		started, err := r.beginRegistration(ctx, userID, authTime)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "beginPasskeyRegistration")
		}

		return &ceremonyOutput{Body: PasskeyCeremony{
			Options:       started.Options,
			CeremonyToken: started.Token,
		}}, nil
	})
}

func (r *controller) finishPasskeyRegistration(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "finish-passkey-registration",
		Summary:       "Finish adding a passkey",
		Description:   "Verify the browser's new credential against the ceremony and store it as a passkey.",
		Method:        http.MethodPost,
		Path:          "/v1/passkeys/register/finish",
		Tags:          []string{_passkeysTag},
		DefaultStatus: http.StatusCreated,
		Security:      []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:        apierr.CommandErrors(http.StatusBadRequest, http.StatusUnauthorized, http.StatusConflict),
	}, func(ctx context.Context, input *finishPasskeyRegistrationInput) (*passkeyOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		passkey, err := r.finishRegistration(ctx, userID, input.CeremonyToken, input.Name, input.Body)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "finishPasskeyRegistration")
		}

		return &passkeyOutput{Body: passkey}, nil
	})
}

func (r *controller) listPasskeys(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-passkeys",
		Summary:     "List passkeys",
		Description: "List the current user's passkeys, oldest first",
		Method:      http.MethodGet,
		Path:        "/v1/passkeys",
		Tags:        []string{_passkeysTag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized),
	}, func(ctx context.Context, _ *listPasskeysInput) (*listPasskeysOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		passkeys, err := r.list(ctx, userID)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "listPasskeys")
		}

		return &listPasskeysOutput{Body: NewPasskeyList(passkeys)}, nil
	})
}

type passkeyPathInput struct {
	ID string `path:"id" example:"AQIDBA" doc:"The passkey's ID, unpadded base64url"`
}

type renamePasskeyInput struct {
	ID   string `path:"id" example:"AQIDBA" doc:"The passkey's ID, unpadded base64url"`
	Body RenamePasskey
}

type deletePasskeyOutput struct {
	Status int
}

func (r *controller) renamePasskey(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "rename-passkey",
		Summary:     "Rename a passkey",
		Description: "Rename one of the current user's passkeys. Another user's passkey is not found.",
		Method:      http.MethodPatch,
		Path:        "/v1/passkeys/{id}",
		Tags:        []string{_passkeysTag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.CommandErrors(http.StatusUnauthorized, http.StatusNotFound),
	}, func(ctx context.Context, input *renamePasskeyInput) (*passkeyOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		passkey, err := r.rename(ctx, userID, input.ID, input.Body.Name)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "renamePasskey")
		}

		return &passkeyOutput{Body: passkey}, nil
	})
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
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		err := r.delete(ctx, userID, input.ID)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "deletePasskey")
		}

		return &deletePasskeyOutput{Status: http.StatusNoContent}, nil
	})
}

type beginPasskeyLoginInput struct{}

type finishPasskeyLoginInput struct {
	CeremonyToken string `header:"X-Ceremony-Token" required:"true" minLength:"1" doc:"The ceremony_token from begin"`

	Body PasskeyCredential
}

type finishPasskeyLoginOutput struct {
	Body response.Token
}

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
	}, func(ctx context.Context, _ *beginPasskeyLoginInput) (*ceremonyOutput, error) {
		started, err := r.beginLogin(ctx)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "beginPasskeyLogin")
		}

		return &ceremonyOutput{Body: PasskeyCeremony{
			Options:       started.Options,
			CeremonyToken: started.Token,
		}}, nil
	})
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
		token, err := r.finishLogin(ctx, input.CeremonyToken, input.Body)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "finishPasskeyLogin")
		}

		return &finishPasskeyLoginOutput{Body: response.Token{Token: token}}, nil
	})
}
