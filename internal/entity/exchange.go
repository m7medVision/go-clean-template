package entity

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// _resultScale is the number of decimal places a converted amount is rounded to.
const _resultScale = 2

// Conversion is an amount converted from one currency to another at a reference rate.
// Amounts are exact decimals: money never passes through float64.
type Conversion struct {
	From     string
	To       string
	Amount   decimal.Decimal
	Rate     decimal.Decimal
	Result   decimal.Decimal
	RateDate time.Time
}

// ExchangeHistory -.
type ExchangeHistory struct {
	History []Conversion
}

// NewConversion validates a conversion request. Currency codes are trimmed and uppercased and
// must be three letters (ISO 4217 shape); the two codes must differ and the amount must be
// positive. Whether a code is a currency the rate provider knows is checked by the provider.
func NewConversion(from, to string, amount decimal.Decimal) (Conversion, error) {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))

	if !validCurrencyCode(from) || !validCurrencyCode(to) || from == to || !amount.IsPositive() {
		return Conversion{}, ErrInvalidConversion
	}

	return Conversion{From: from, To: to, Amount: amount}, nil
}

// ApplyRate sets a rate published on date: Result is Amount × Rate, rounded half-even.
func (c *Conversion) ApplyRate(rate decimal.Decimal, date time.Time) {
	c.Rate = rate
	c.RateDate = date
	c.Result = c.Amount.Mul(rate).RoundBank(_resultScale)
}

func validCurrencyCode(code string) bool {
	const length = 3

	if len(code) != length {
		return false
	}

	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return false
		}
	}

	return true
}
