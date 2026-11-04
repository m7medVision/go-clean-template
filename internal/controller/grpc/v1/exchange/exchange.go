package exchange

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Convert -.
func (c *Controller) Convert(ctx context.Context, req *v1.ConvertRequest) (*v1.ConversionResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	amount, err := decimal.NewFromString(req.GetAmount())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid amount")
	}

	written, err := cqrs.SendCommand[cqrs.Written](ctx, c.cmds, command.ConvertCurrency{
		UserID: userID, From: req.GetFrom(), To: req.GetTo(), Amount: amount,
	})
	if err != nil {
		return nil, c.fail(err, "Convert")
	}

	conversion, err := cqrs.SendQuery[entity.Conversion](ctx, c.qs,
		query.GetConversion{UserID: userID, ConversionID: written.ID})
	if err != nil {
		return nil, c.fail(err, "Convert")
	}

	return NewConversionResponse(&conversion), nil
}

// GetHistory -.
func (c *Controller) GetHistory(
	ctx context.Context, req *v1.GetExchangeHistoryRequest,
) (*v1.ExchangeHistoryResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	history, err := cqrs.SendQuery[entity.ExchangeHistory](ctx, c.qs,
		query.GetExchangeHistory{UserID: userID, Limit: int(req.GetLimit()), Offset: int(req.GetOffset())})
	if err != nil {
		return nil, c.fail(err, "GetHistory")
	}

	return NewExchangeHistoryResponse(history), nil
}
