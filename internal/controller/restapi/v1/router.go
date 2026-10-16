package v1

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
)

const (
	_bearerAuthScheme = "BearerAuth"
	_authTag          = "auth"
	_usersTag         = "users"
	_tasksTag         = "tasks"
	_translationTag   = "translation"
)

// NewRoutes registers every v1 REST operation on the Huma API and sets up the
// security scheme reproducing the BearerAuth header of the previous transport.
func NewRoutes(api huma.API, t usecase.Translation, u usecase.User, tk usecase.Task, l logger.Interface) {
	api.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		_bearerAuthScheme: {
			Type: "apiKey",
			In:   "header",
			Name: "Authorization",
		},
	}

	r := &V1{t: t, u: u, tk: tk, l: l}

	r.register(api)
	r.login(api)
	r.profile(api)

	r.createTask(api)
	r.listTasks(api)
	r.getTask(api)
	r.updateTask(api)
	r.transitionTask(api)
	r.deleteTask(api)

	r.history(api)
	r.doTranslate(api)
}
