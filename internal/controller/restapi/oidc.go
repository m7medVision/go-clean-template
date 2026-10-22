package restapi

import (
	"github.com/danielgtaylor/huma/v2"
	v1 "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// WithOIDC serves the SSO status and sign-in.
func WithOIDC(oc usecase.OIDC) Option {
	return func(api huma.API, l logger.Interface) {
		v1.NewOIDCRoutes(api, oc, l)
	}
}
