// Package exchange serves the v1 currency conversion operations over REST.
package exchange

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

const _exchangeTag = "exchange"

type controller struct {
	e usecase.Exchange
	l logger.Interface
}

// NewRoutes registers the exchange operations.
func NewRoutes(api huma.API, e usecase.Exchange, l logger.Interface) {
	r := &controller{e: e, l: l}

	r.convert(api)
	r.exchangeHistory(api)
}
