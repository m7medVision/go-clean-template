package graphql

import (
	"log/slog"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/exchange"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/generated"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/task"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// The root Query and Mutation embed each domain's resolver, so the domain's methods become theirs.
// Go names an embedded field after its type, and every domain calls its types Query and Mutation,
// so each is embedded through an alias that gives its field a distinct name. A schema field no
// domain resolves fails the build with "missing method <Field>".
type (
	taskQuery     = task.Query
	taskMutation  = task.Mutation
	exchangeQuery = exchange.Query
)

type query struct {
	taskQuery
	exchangeQuery
}

type mutation struct {
	taskMutation
}

// resolver is the root gqlgen asks for the Query and Mutation resolvers.
type resolver struct {
	query    query
	mutation mutation
}

// newResolver fills in each domain's resolvers, one statement per domain.
func newResolver(b cqrs.Buses, l *slog.Logger) *resolver {
	r := &resolver{}
	r.query.taskQuery, r.mutation.taskMutation = task.New(b, l)
	r.query.exchangeQuery = exchange.New(b, l)

	return r
}

func (r *resolver) Query() generated.QueryResolver { return r.query }

func (r *resolver) Mutation() generated.MutationResolver { return r.mutation }
