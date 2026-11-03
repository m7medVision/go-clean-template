package exchange_test

import (
	"context"
	"net/http"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/shopspring/decimal"
)

type fakeExchange struct {
	usecase.Exchange

	converted entity.Conversion
	err       error

	// gotLimit and gotOffset record what History was called with.
	gotLimit, gotOffset int
}

func (f *fakeExchange) Convert(context.Context, string, string, string, decimal.Decimal) (entity.Conversion, error) {
	return f.converted, f.err
}

func (f *fakeExchange) History(_ context.Context, _ string, limit, offset int) (entity.ExchangeHistory, error) {
	f.gotLimit, f.gotOffset = limit, offset

	return entity.ExchangeHistory{History: []entity.Conversion{f.converted}, Total: 11}, f.err
}

// newRouter serves the REST router over e; the use cases a exchange test never reaches are nil.
func newRouter(cfg *config.Config, e usecase.Exchange) http.Handler {
	return resttest.NewRouter(cfg, e, nil, nil)
}
