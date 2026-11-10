package app

import (
	restflags "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/flags"
	"github.com/m7medvision/go-clean-template/internal/usecase/flags"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// flagsModule answers the caller's feature flags, over REST and GraphQL (no gRPC: only the web
// client asks).
func flagsModule() module {
	return module{
		handlers: func(d *deps, b cqrs.Buses) error {
			flags.Register(b.Queries, flags.Deps{Flags: d.flags})

			return nil
		},
		rest: restflags.NewRoutes,
	}
}
