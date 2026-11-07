// Package exchange serves ExchangeService (docs/proto/v1/exchange.proto) over gRPC.
package exchange

import (
	"errors"
	"log/slog"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Controller -.
type Controller struct {
	v1.UnimplementedExchangeServiceServer

	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

// New -.
func New(b cqrs.Buses, l *slog.Logger) v1.ExchangeServiceServer {
	return &Controller{cmds: b.Commands, qs: b.Queries, l: l}
}

// fail maps a failed command or query to its status: first what any command can fail with, then
// the exchange errors, and anything else to a logged Internal.
func (c *Controller) fail(err error, op string) error {
	if shared := rpcerr.Shared(err); shared != nil {
		c.l.Warn("grpc - v1 - "+op, "error", err)

		return shared
	}

	switch {
	case errors.Is(err, entity.ErrInvalidConversion):
		return status.Error(codes.InvalidArgument, "invalid conversion")
	case errors.Is(err, entity.ErrUnsupportedCurrency):
		return status.Error(codes.InvalidArgument, "unsupported currency")
	default:
		c.l.Error("grpc - v1 - "+op, "error", err)

		return status.Error(codes.Internal, "internal server error")
	}
}
