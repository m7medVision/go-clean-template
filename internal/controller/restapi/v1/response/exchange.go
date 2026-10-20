package response

import (
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

// _resultPlaces matches the scale entity.Conversion rounds Result to.
const _resultPlaces = 2

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

// ExchangeHistory -.
type ExchangeHistory struct {
	History []Conversion `json:"history"`
	Total   int          `json:"total" example:"42"`
}

// NewConversion -.
func NewConversion(c *entity.Conversion) Conversion {
	return Conversion{
		From:     c.From,
		To:       c.To,
		Amount:   c.Amount.String(),
		Rate:     c.Rate.String(),
		Result:   c.Result.StringFixed(_resultPlaces),
		RateDate: c.RateDate.Format(time.DateOnly),
	}
}

// NewExchangeHistory -.
func NewExchangeHistory(h entity.ExchangeHistory) ExchangeHistory {
	history := make([]Conversion, 0, len(h.History))
	for i := range h.History {
		history = append(history, NewConversion(&h.History[i]))
	}

	return ExchangeHistory{History: history, Total: h.Total}
}
