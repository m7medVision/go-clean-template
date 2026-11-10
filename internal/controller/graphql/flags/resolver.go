// Package flags resolves the feature-flag fields of the GraphQL schema.
package flags

import (
	"log/slog"

	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type Query struct {
	qs *cqrs.Queries
	l  *slog.Logger
}

func New(b cqrs.Buses, l *slog.Logger) Query {
	return Query{qs: b.Queries, l: l}
}
