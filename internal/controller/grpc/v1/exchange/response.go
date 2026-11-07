package exchange

import (
	"time"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/response"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

func NewConversionResponse(c *entity.Conversion) *v1.ConversionResponse {
	return &v1.ConversionResponse{
		From:     c.From,
		To:       c.To,
		Amount:   c.Amount.String(),
		Rate:     c.Rate.String(),
		Result:   c.ResultString(),
		RateDate: c.RateDate.Format(time.DateOnly),
	}
}

func NewExchangeHistoryResponse(h entity.List[entity.Conversion]) *v1.ExchangeHistoryResponse {
	history := make([]*v1.ConversionResponse, len(h.Items))
	for i := range h.Items {
		history[i] = NewConversionResponse(&h.Items[i])
	}

	return &v1.ExchangeHistoryResponse{History: history, Total: response.Count(h.Total)}
}
