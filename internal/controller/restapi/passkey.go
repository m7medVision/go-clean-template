package restapi

import (
	"github.com/danielgtaylor/huma/v2"
	v1 "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// WithPasskeys serves passkey sign-in and passkey management.
func WithPasskeys() Option {
	return func(api huma.API, b cqrs.Buses, l logger.Interface) {
		v1.NewPasskeyRoutes(api, b, l)
	}
}
