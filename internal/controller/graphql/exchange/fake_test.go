package exchange_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/graphqltest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type fakeExchange struct {
	usecase.Exchange

	history             entity.ExchangeHistory
	err                 error
	called              bool
	gotLimit, gotOffset int
}

func (f *fakeExchange) History(_ context.Context, _ string, limit, offset int) (entity.ExchangeHistory, error) {
	f.called = true
	f.gotLimit, f.gotOffset = limit, offset

	return f.history, f.err
}

// do sends query to the GraphQL handler over ex, signed in as graphqltest.UserID when authenticated.
func do(t *testing.T, ex *fakeExchange, authenticated bool, query string) graphqltest.Response {
	t.Helper()

	token := ""
	if authenticated {
		token = graphqltest.Token(t)
	}

	return graphqltest.Post(t, cqrs.Buses{}, ex, token, query, nil)
}
