package exchange

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/model"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/paging"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// _maxLimit is the largest page exchangeHistory serves, as the schema documents.
const _maxLimit = 100

// ExchangeHistory resolves Query.exchangeHistory.
func (q Query) ExchangeHistory(ctx context.Context, limit, offset int) (*model.ConversionList, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	if !paging.ValidUpTo(limit, offset, _maxLimit) {
		return nil, gqlerr.InvalidInput()
	}

	history, err := cqrs.SendQuery(ctx, q.qs,
		query.GetExchangeHistory{UserID: caller.UserID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, gqlerr.Map(ctx, q.l, err, "exchangeHistory")
	}

	return newConversionList(history), nil
}
