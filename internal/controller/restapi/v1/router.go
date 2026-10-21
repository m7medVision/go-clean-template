package v1

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

const (
	_bearerAuthScheme = "BearerAuth"
	_authTag          = "auth"
	_usersTag         = "users"
	_tasksTag         = "tasks"
	_exchangeTag      = "exchange"
	_passkeysTag      = "passkeys"
)

// NewRoutes registers every v1 REST operation on the Huma API and sets up the
// security scheme reproducing the BearerAuth header of the previous transport.
func NewRoutes(
	api huma.API,
	e usecase.Exchange,
	u usecase.User,
	tk usecase.Task,
	pk usecase.Passkey,
	oc usecase.OIDC,
	l logger.Interface,
) {
	api.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		_bearerAuthScheme: {
			Type: "apiKey",
			In:   "header",
			Name: "Authorization",
		},
	}

	r := &V1{e: e, u: u, tk: tk, pk: pk, oc: oc, l: l}

	r.register(api)
	r.login(api)
	r.profile(api)
	r.beginPasskeyLogin(api)
	r.finishPasskeyLogin(api)
	r.oidcStatus(api)
	r.beginOIDC(api)
	r.finishOIDC(api)

	r.beginPasskeyRegistration(api)
	r.finishPasskeyRegistration(api)
	r.listPasskeys(api)
	r.renamePasskey(api)
	r.deletePasskey(api)

	r.createTask(api)
	r.listTasks(api)
	r.listAllTasks(api)
	r.getTask(api)
	r.updateTask(api)
	r.transitionTask(api)
	r.deleteTask(api)

	r.convert(api)
	r.exchangeHistory(api)
}
