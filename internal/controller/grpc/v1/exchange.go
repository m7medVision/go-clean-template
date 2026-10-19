package v1

import (
	"context"
	"errors"

	v1 "github.com/evrone/go-clean-template/docs/proto/v1"
	grpcmw "github.com/evrone/go-clean-template/internal/controller/grpc/middleware"
	"github.com/evrone/go-clean-template/internal/controller/grpc/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Convert -.
func (c *ExchangeController) Convert(ctx context.Context, req *v1.ConvertRequest) (*v1.ConversionResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	amount, err := decimal.NewFromString(req.GetAmount())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid amount")
	}

	conversion, err := c.e.Convert(ctx, userID, req.GetFrom(), req.GetTo(), amount)
	if err != nil {
		if errors.Is(err, entity.ErrInvalidConversion) {
			return nil, status.Error(codes.InvalidArgument, "invalid conversion")
		}

		if errors.Is(err, entity.ErrUnsupportedCurrency) {
			return nil, status.Error(codes.InvalidArgument, "unsupported currency")
		}

		c.l.Error(err, "grpc - v1 - Convert")

		return nil, status.Error(codes.Internal, "internal server error")
	}

	return response.NewConversionResponse(&conversion), nil
}

// GetHistory -.
func (c *ExchangeController) GetHistory(
	ctx context.Context, req *v1.GetExchangeHistoryRequest,
) (*v1.ExchangeHistoryResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	history, err := c.e.History(ctx, userID, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		c.l.Error(err, "grpc - v1 - GetHistory")

		return nil, status.Error(codes.Internal, "internal server error")
	}

	return response.NewExchangeHistoryResponse(history), nil
}
