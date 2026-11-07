package exchange

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetHistory -.
func (c *Controller) GetHistory(
	ctx context.Context, req *v1.GetExchangeHistoryRequest,
) (*v1.ExchangeHistoryResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	history, err := cqrs.SendQuery(ctx, c.qs,
		query.GetExchangeHistory{UserID: userID, Limit: int(req.GetLimit()), Offset: int(req.GetOffset())})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "GetHistory")
	}

	return NewExchangeHistoryResponse(history), nil
}
