package exchange_test

import (
	"context"
	"net/http"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// fakeExchange answers the exchange command and queries on real buses: ConvertCurrency writes
// converted, which GetConversion reads back, and every message fails with err when it is set.
type fakeExchange struct {
	converted entity.Conversion
	err       error

	// gotLimit and gotOffset record what GetExchangeHistory was asked.
	gotLimit, gotOffset int
}

func (f *fakeExchange) buses() cqrs.Buses {
	b := resttest.NewBuses()

	cqrs.HandleCommand(b.Commands, cqrs.HandlerFunc[command.ConvertCurrency, entity.Written](
		func(context.Context, command.ConvertCurrency) (entity.Written, error) {
			return entity.Written{ID: "c1"}, f.err
		}))
	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[query.GetConversion, entity.Conversion](
		func(context.Context, query.GetConversion) (entity.Conversion, error) { return f.converted, f.err }))
	cqrs.HandleQuery(b.Queries, cqrs.HandlerFunc[query.GetExchangeHistory, entity.ExchangeHistory](
		func(_ context.Context, q query.GetExchangeHistory) (entity.ExchangeHistory, error) {
			f.gotLimit, f.gotOffset = q.Limit, q.Offset

			return entity.ExchangeHistory{History: []entity.Conversion{f.converted}, Total: 11}, f.err
		}))

	return b
}

// newRouter serves the REST router over e's buses.
func newRouter(cfg *config.Config, e *fakeExchange) http.Handler {
	return resttest.NewRouter(cfg, e.buses())
}
