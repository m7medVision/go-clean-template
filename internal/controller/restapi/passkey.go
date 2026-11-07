package restapi

import (
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	v1 "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// WithPasskeys serves passkey sign-in and passkey management.
func WithPasskeys() Option {
	return func(api huma.API, b cqrs.Buses, l *slog.Logger) {
		v1.NewPasskeyRoutes(api, b, l)
	}
}
