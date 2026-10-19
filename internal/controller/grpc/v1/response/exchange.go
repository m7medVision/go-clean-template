package response

import (
	"math"
	"time"

	v1 "github.com/evrone/go-clean-template/docs/proto/v1"
	"github.com/evrone/go-clean-template/internal/entity"
)

// _resultPlaces matches the scale entity.Conversion rounds Result to.
const _resultPlaces = 2

// NewConversionResponse -.
func NewConversionResponse(c *entity.Conversion) *v1.ConversionResponse {
	return &v1.ConversionResponse{
		From:     c.From,
		To:       c.To,
		Amount:   c.Amount.String(),
		Rate:     c.Rate.String(),
		Result:   c.Result.StringFixed(_resultPlaces),
		RateDate: c.RateDate.Format(time.DateOnly),
	}
}

// NewExchangeHistoryResponse -.
func NewExchangeHistoryResponse(h entity.ExchangeHistory) *v1.ExchangeHistoryResponse {
	history := make([]*v1.ConversionResponse, len(h.History))
	for i := range h.History {
		history[i] = NewConversionResponse(&h.History[i])
	}

	return &v1.ExchangeHistoryResponse{History: history, Total: clampInt32(h.Total)}
}

// clampInt32 narrows a count to the proto's int32.
func clampInt32(n int) int32 {
	if n < 0 {
		return 0
	}

	if n > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(n)
}
