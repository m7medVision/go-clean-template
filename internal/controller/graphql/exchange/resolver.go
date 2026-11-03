// Package exchange resolves the exchange fields of the GraphQL schema (schema/exchange.graphqls).
package exchange

import (
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// Query resolves the exchange fields of Query.
type Query struct {
	exchange usecase.Exchange
	l        logger.Interface
}

// New returns the exchange resolver for Query.
func New(uc usecase.Exchange, l logger.Interface) Query {
	return Query{exchange: uc, l: l}
}
