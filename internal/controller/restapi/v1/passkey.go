package v1

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// NewPasskeyRoutes registers the passkey sign-in and passkey management operations.
func NewPasskeyRoutes(api huma.API, pk usecase.Passkey, l logger.Interface) {
	r := &V1{pk: pk, l: l}

	r.beginPasskeyLogin(api)
	r.finishPasskeyLogin(api)

	r.beginPasskeyRegistration(api)
	r.finishPasskeyRegistration(api)
	r.listPasskeys(api)
	r.renamePasskey(api)
	r.deletePasskey(api)
}

// _reauthRequired is the problem detail of a register/begin refused for a stale sign-in. Clients
// match on it to ask for the password again, so it must not change.
const _reauthRequired = "reauthentication required"

type beginPasskeyRegistrationInput struct{}

type beginPasskeyRegistrationOutput struct {
	Body response.PasskeyCeremony
}

type finishPasskeyRegistrationInput struct {
	CeremonyToken string `header:"X-Ceremony-Token" required:"true" minLength:"1" doc:"The ceremony_token from begin"`
	Name          string `query:"name" maxLength:"64" doc:"A label for the passkey; defaults to \"Passkey\""`

	Body request.PasskeyCredential
}

type finishPasskeyRegistrationOutput struct {
	Body entity.Passkey
}

type listPasskeysInput struct{}

type listPasskeysOutput struct {
	Body response.PasskeyList
}

func (r *V1) beginPasskeyRegistration(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "begin-passkey-registration",
		Summary:     "Begin adding a passkey",
		Description: "Start a WebAuthn registration ceremony for the current user. Pass `options` to " +
			"navigator.credentials.create and send the result to finish with `ceremony_token` in the " +
			"X-Ceremony-Token header. The caller must have signed in recently (WEBAUTHN_REAUTH_MAX_AGE); " +
			"otherwise the answer is a 401 whose detail is \"" + _reauthRequired + "\": sign in again and retry.",
		Method:   http.MethodPost,
		Path:     "/v1/passkeys/register/begin",
		Tags:     []string{_passkeysTag},
		Security: []map[string][]string{{_bearerAuthScheme: {}}},
		Errors:   []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError},
	}, func(ctx context.Context, _ *beginPasskeyRegistrationInput) (*beginPasskeyRegistrationOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		// A missing auth time is zero, which the use case treats as a sign-in too old to add a passkey.
		authTime, _ := middleware.AuthTime(ctx)

		options, token, err := r.pk.BeginRegistration(ctx, userID, authTime)
		if err != nil {
			r.l.Error(err, "restapi - v1 - beginPasskeyRegistration")

			if errors.Is(err, entity.ErrPasskeyReauthRequired) {
				return nil, huma.Error401Unauthorized(_reauthRequired)
			}

			if errors.Is(err, entity.ErrUserNotFound) {
				return nil, huma.Error404NotFound("user not found")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &beginPasskeyRegistrationOutput{Body: response.PasskeyCeremony{
			Options:       options,
			CeremonyToken: token,
		}}, nil
	})
}

func (r *V1) finishPasskeyRegistration(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "finish-passkey-registration",
		Summary:       "Finish adding a passkey",
		Description:   "Verify the browser's new credential against the ceremony and store it as a passkey.",
		Method:        http.MethodPost,
		Path:          "/v1/passkeys/register/finish",
		Tags:          []string{_passkeysTag},
		DefaultStatus: http.StatusCreated,
		Security:      []map[string][]string{{_bearerAuthScheme: {}}},
		Errors: []int{
			http.StatusBadRequest, http.StatusUnauthorized, http.StatusConflict, http.StatusInternalServerError,
		},
	}, func(ctx context.Context, input *finishPasskeyRegistrationInput) (*finishPasskeyRegistrationOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		passkey, err := r.pk.FinishRegistration(ctx, userID, input.CeremonyToken, input.Name, input.Body)
		if err != nil {
			r.l.Error(err, "restapi - v1 - finishPasskeyRegistration")

			if errors.Is(err, entity.ErrPasskeyCeremonyInvalid) {
				return nil, huma.Error400BadRequest("invalid passkey ceremony")
			}

			if errors.Is(err, entity.ErrPasskeyExists) {
				return nil, huma.Error409Conflict("passkey already registered")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &finishPasskeyRegistrationOutput{Body: passkey}, nil
	})
}

func (r *V1) listPasskeys(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-passkeys",
		Summary:     "List passkeys",
		Description: "List the current user's passkeys, oldest first",
		Method:      http.MethodGet,
		Path:        "/v1/passkeys",
		Tags:        []string{_passkeysTag},
		Security:    []map[string][]string{{_bearerAuthScheme: {}}},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, func(ctx context.Context, _ *listPasskeysInput) (*listPasskeysOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		passkeys, err := r.pk.List(ctx, userID)
		if err != nil {
			r.l.Error(err, "restapi - v1 - listPasskeys")

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &listPasskeysOutput{Body: response.NewPasskeyList(passkeys)}, nil
	})
}

type passkeyPathInput struct {
	ID string `path:"id" example:"AQIDBA" doc:"The passkey's ID, unpadded base64url"`
}

type renamePasskeyInput struct {
	ID   string `path:"id" example:"AQIDBA" doc:"The passkey's ID, unpadded base64url"`
	Body request.RenamePasskey
}

type renamePasskeyOutput struct {
	Body entity.Passkey
}

type deletePasskeyOutput struct {
	Status int
}

func (r *V1) renamePasskey(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "rename-passkey",
		Summary:     "Rename a passkey",
		Description: "Rename one of the current user's passkeys. Another user's passkey is not found.",
		Method:      http.MethodPatch,
		Path:        "/v1/passkeys/{id}",
		Tags:        []string{_passkeysTag},
		Security:    []map[string][]string{{_bearerAuthScheme: {}}},
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError},
	}, func(ctx context.Context, input *renamePasskeyInput) (*renamePasskeyOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		passkey, err := r.pk.Rename(ctx, userID, input.ID, input.Body.Name)
		if err != nil {
			r.l.Error(err, "restapi - v1 - renamePasskey")

			if errors.Is(err, entity.ErrPasskeyNotFound) {
				return nil, huma.Error404NotFound("passkey not found")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &renamePasskeyOutput{Body: passkey}, nil
	})
}

func (r *V1) deletePasskey(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "delete-passkey",
		Summary:     "Delete a passkey",
		Description: "Delete one of the current user's passkeys; it can no longer sign in. " +
			"Another user's passkey is not found.",
		Method:        http.MethodDelete,
		Path:          "/v1/passkeys/{id}",
		Tags:          []string{_passkeysTag},
		Security:      []map[string][]string{{_bearerAuthScheme: {}}},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError},
	}, func(ctx context.Context, input *passkeyPathInput) (*deletePasskeyOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		err := r.pk.Delete(ctx, userID, input.ID)
		if err != nil {
			r.l.Error(err, "restapi - v1 - deletePasskey")

			if errors.Is(err, entity.ErrPasskeyNotFound) {
				return nil, huma.Error404NotFound("passkey not found")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &deletePasskeyOutput{Status: http.StatusNoContent}, nil
	})
}

type beginPasskeyLoginInput struct{}

type beginPasskeyLoginOutput struct {
	Body response.PasskeyCeremony
}

type finishPasskeyLoginInput struct {
	CeremonyToken string `header:"X-Ceremony-Token" required:"true" minLength:"1" doc:"The ceremony_token from begin"`

	Body request.PasskeyCredential
}

type finishPasskeyLoginOutput struct {
	Body response.Token
}

func (r *V1) beginPasskeyLogin(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "begin-passkey-login",
		Summary:     "Begin passkey login",
		Description: "Start a WebAuthn login ceremony for any registered passkey. Pass `options` to " +
			"navigator.credentials.get and send the result to finish with `ceremony_token` in the " +
			"X-Ceremony-Token header.",
		Method: http.MethodPost,
		Path:   "/v1/auth/passkey/begin",
		Tags:   []string{_authTag},
		Errors: []int{http.StatusInternalServerError},
	}, func(ctx context.Context, _ *beginPasskeyLoginInput) (*beginPasskeyLoginOutput, error) {
		options, token, err := r.pk.BeginLogin(ctx)
		if err != nil {
			r.l.Error(err, "restapi - v1 - beginPasskeyLogin")

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &beginPasskeyLoginOutput{Body: response.PasskeyCeremony{
			Options:       options,
			CeremonyToken: token,
		}}, nil
	})
}

func (r *V1) finishPasskeyLogin(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "finish-passkey-login",
		Summary:     "Finish passkey login",
		Description: "Verify the browser's assertion against the ceremony and get a JWT token. " +
			"A ceremony completes once; finishing it again is a conflict.",
		Method: http.MethodPost,
		Path:   "/v1/auth/passkey/finish",
		Tags:   []string{_authTag},
		Errors: []int{
			http.StatusBadRequest, http.StatusUnauthorized, http.StatusConflict, http.StatusInternalServerError,
		},
	}, func(ctx context.Context, input *finishPasskeyLoginInput) (*finishPasskeyLoginOutput, error) {
		token, err := r.pk.FinishLogin(ctx, input.CeremonyToken, input.Body)
		if err != nil {
			r.l.Error(err, "restapi - v1 - finishPasskeyLogin")

			if errors.Is(err, entity.ErrPasskeyCeremonyInvalid) {
				return nil, huma.Error401Unauthorized("invalid passkey login")
			}

			if errors.Is(err, entity.ErrPasskeyChallengeUsed) {
				return nil, huma.Error409Conflict("passkey login already completed")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &finishPasskeyLoginOutput{Body: response.Token{Token: token}}, nil
	})
}
