package exchange

import (
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

// Conversion is entity.Conversion with its decimals as strings: decimal.Decimal has no schema of
// its own, and a JSON number would lose precision in most clients.
type Conversion struct {
	From     string `json:"from"      example:"EUR"`
	To       string `json:"to"        example:"USD"`
	Amount   string `json:"amount"    example:"100.1"`
	Rate     string `json:"rate"      example:"1.1225"`
	Result   string `json:"result"    example:"112.36"`
	RateDate string `json:"rate_date" example:"2026-10-19" format:"date"`
}

type ExchangeHistory struct { //nolint:revive // Huma names the OpenAPI schema after this type, so the name is API contract
	History []Conversion `json:"history"`
	Total   int          `json:"total" example:"42"`
}

func NewConversion(c *entity.Conversion) Conversion {
	return Conversion{
		From:     c.From,
		To:       c.To,
		Amount:   c.Amount.String(),
		Rate:     c.Rate.String(),
		Result:   c.ResultString(),
		RateDate: c.RateDate.Format(time.DateOnly),
	}
}

func NewExchangeHistory(h entity.List[entity.Conversion]) ExchangeHistory {
	history := make([]Conversion, 0, len(h.Items))
	for i := range h.Items {
		history = append(history, NewConversion(&h.Items[i]))
	}

	return ExchangeHistory{History: history, Total: h.Total}
}
