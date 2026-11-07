// Package exchange serves ExchangeService (docs/proto/v1/exchange.proto) over gRPC.
package exchange

import (
	"log/slog"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
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
