package v1

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/exchange"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/oidc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/passkey"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/task"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/user"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// NewRoutes registers the core v1 REST operations on the Huma API and sets up the
// security scheme reproducing the BearerAuth header of the previous transport. Each domain
// registers its own operations from its folder; optional features register theirs afterwards
// through NewPasskeyRoutes and NewOIDCRoutes.
func NewRoutes(api huma.API, b cqrs.Buses, e usecase.Exchange, u usecase.User, l logger.Interface) {
	api.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		apidoc.BearerAuthScheme: {
			Type: "apiKey",
			In:   "header",
			Name: "Authorization",
		},
	}

	user.NewRoutes(api, u, l)
	task.NewRoutes(api, b, l)
	exchange.NewRoutes(api, e, l)
}

// NewPasskeyRoutes registers the passkey sign-in and passkey management operations.
func NewPasskeyRoutes(api huma.API, pk usecase.Passkey, l logger.Interface) {
	passkey.NewRoutes(api, pk, l)
}

// NewOIDCRoutes registers the SSO status and sign-in operations.
func NewOIDCRoutes(api huma.API, oc usecase.OIDC, l logger.Interface) {
	oidc.NewRoutes(api, oc, l)
}
