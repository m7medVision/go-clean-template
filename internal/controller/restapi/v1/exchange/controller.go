// Package exchange serves the v1 currency conversion operations over REST.
package exchange

import (
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

const _tag = "exchange"

type controller struct {
	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

func NewRoutes(api huma.API, b cqrs.Buses, l *slog.Logger) {
	r := &controller{cmds: b.Commands, qs: b.Queries, l: l}

	r.convert(api)
	r.exchangeHistory(api)
}
