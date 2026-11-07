package restapi

import (
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	v1 "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// WithOIDC serves the SSO status and sign-in.
func WithOIDC() Option {
	return func(api huma.API, b cqrs.Buses, l *slog.Logger) {
		v1.NewOIDCRoutes(api, b, l)
	}
}
