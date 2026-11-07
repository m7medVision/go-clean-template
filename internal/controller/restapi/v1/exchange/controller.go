// Package exchange serves the v1 currency conversion operations over REST.
package exchange

import (
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

const _exchangeTag = "exchange"

type controller struct {
	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

// NewRoutes registers the exchange operations.
func NewRoutes(api huma.API, b cqrs.Buses, l *slog.Logger) {
	r := &controller{cmds: b.Commands, qs: b.Queries, l: l}

	r.convert(api)
	r.exchangeHistory(api)
}

// problem maps a failed command or query to its REST problem: first what any command can fail
// with, then the exchange errors, and anything else to a logged 500.
func (r *controller) problem(err error, op string) error {
	if shared := apierr.Shared(err); shared != nil {
		r.l.Warn("restapi - v1 - "+op, "error", err)

		return shared
	}

	switch {
	case errors.Is(err, entity.ErrInvalidConversion):
		return huma.Error400BadRequest("invalid conversion")
	case errors.Is(err, entity.ErrUnsupportedCurrency):
		return huma.Error400BadRequest("unsupported currency")
	default:
		r.l.Error("restapi - v1 - "+op, "error", err)

		return huma.Error500InternalServerError("internal server error")
	}
}
