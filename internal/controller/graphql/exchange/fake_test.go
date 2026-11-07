package exchange_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/graphqltest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// fakeExchange answers GetExchangeHistory on real buses, and keeps what it was asked.
type fakeExchange struct {
	history             entity.List[entity.Conversion]
	err                 error
	called              bool
	gotLimit, gotOffset int
}

func (f *fakeExchange) buses() cqrs.Buses {
	qs := cqrs.NewQueries()
	cqrs.HandleQuery(qs, cqrs.HandlerFunc[query.GetExchangeHistory, entity.List[entity.Conversion]](
		func(_ context.Context, q query.GetExchangeHistory) (entity.List[entity.Conversion], error) {
			f.called = true
			f.gotLimit, f.gotOffset = q.Limit, q.Offset

			return f.history, f.err
		}))

	return cqrs.Buses{Commands: cqrs.NewCommands(), Queries: qs}
}

// do sends gql to the GraphQL handler over ex, signed in as graphqltest.UserID when authenticated.
func do(t *testing.T, ex *fakeExchange, authenticated bool, gql string) graphqltest.Response {
	t.Helper()

	token := ""
	if authenticated {
		token = graphqltest.Token(t)
	}

	return graphqltest.Post(t, ex.buses(), token, gql, nil)
}
