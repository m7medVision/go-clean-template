// Package exchange serves ExchangeService (docs/proto/v1/exchange.proto) over gRPC.
package exchange

import (
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// Controller -.
type Controller struct {
	v1.UnimplementedExchangeServiceServer

	e usecase.Exchange
	l logger.Interface
}

// New -.
func New(e usecase.Exchange, l logger.Interface) v1.ExchangeServiceServer {
	return &Controller{e: e, l: l}
}
