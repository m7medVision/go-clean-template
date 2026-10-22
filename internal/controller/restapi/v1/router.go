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
	_passkeysTag      = "passkeys" // scaffold:passkeys
)

// NewRoutes registers the core v1 REST operations on the Huma API and sets up the
// security scheme reproducing the BearerAuth header of the previous transport. Optional
// features register their own operations afterwards, each from its own file in this package.
func NewRoutes(api huma.API, e usecase.Exchange, u usecase.User, tk usecase.Task, l logger.Interface) {
	api.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		_bearerAuthScheme: {
			Type: "apiKey",
			In:   "header",
			Name: "Authorization",
		},
	}

	r := &V1{e: e, u: u, tk: tk, l: l}

	r.register(api)
	r.login(api)
	r.profile(api)

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
