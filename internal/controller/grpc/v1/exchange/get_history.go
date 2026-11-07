package exchange

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (c *Controller) GetHistory(
	ctx context.Context, req *v1.GetExchangeHistoryRequest,
) (*v1.ExchangeHistoryResponse, error) {
	userID := authn.MustCaller(ctx).UserID

	history, err := cqrs.SendQuery(ctx, c.qs,
		query.GetExchangeHistory{UserID: userID, Limit: int(req.GetLimit()), Offset: int(req.GetOffset())})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "GetHistory")
	}

	return NewExchangeHistoryResponse(history), nil
}
