// Package exchange resolves the exchange fields of the GraphQL schema (schema/exchange.graphqls).
package exchange

import (
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// Query resolves the exchange fields of Query.
type Query struct {
	qs *cqrs.Queries
	l  logger.Interface
}

// New returns the exchange resolver for Query.
func New(b cqrs.Buses, l logger.Interface) Query {
	return Query{qs: b.Queries, l: l}
}
