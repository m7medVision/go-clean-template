// Package flags serves the v1 feature-flag operations over REST.
package flags

import (
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

const _tag = "flags"

type controller struct {
	qs *cqrs.Queries
	l  *slog.Logger
}

func NewRoutes(api huma.API, b cqrs.Buses, l *slog.Logger) {
	r := &controller{qs: b.Queries, l: l}

	r.getFlags(api)
}
