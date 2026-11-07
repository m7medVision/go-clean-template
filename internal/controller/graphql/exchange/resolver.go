// Package exchange resolves the exchange fields of the GraphQL schema: each field in the file
// named as its schema file is (exchange_history.go resolves schema/exchange_history.graphqls).
package exchange

import (
	"log/slog"

	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Query resolves the exchange fields of Query.
type Query struct {
	qs *cqrs.Queries
	l  *slog.Logger
}

// New returns the exchange resolver for Query.
func New(b cqrs.Buses, l *slog.Logger) Query {
	return Query{qs: b.Queries, l: l}
}
