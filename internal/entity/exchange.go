package entity

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrInvalidConversion   = newError(KindInvalid, "invalid conversion")
	ErrConversionNotFound  = newError(KindNotFound, "conversion not found")
	ErrUnsupportedCurrency = newError(KindInvalid, "unsupported currency")
	ErrInvalidAmount       = newError(KindInvalid, fmt.Sprintf(
		"amount must be positive, with at most %d digits before the point and %d after it", MaxAmountDigits, MaxAmountScale))
)

const (
	// _resultScale is the number of decimal places a converted amount is rounded to.
	_resultScale = 2
	// MaxAmountDigits and MaxAmountScale bound an amount to convert: at most 15 digits before the
	// point and 8 after it.
	MaxAmountDigits = 15
	MaxAmountScale  = 8
)

// Conversion is an amount converted from one currency to another at a reference rate, for UserID
// (none while it is being made). Amounts are exact decimals: money never passes through float64.
type Conversion struct {
	UserID   string
	From     string
	To       string
	Amount   decimal.Decimal
	Rate     decimal.Decimal
	Result   decimal.Decimal
	RateDate time.Time
}

// NewConversion validates a conversion request. Currency codes are trimmed and uppercased and
// must be three letters (ISO 4217 shape), and the two codes must differ (else ErrInvalidConversion).
// The amount must be positive and fit MaxAmountDigits and MaxAmountScale (else ErrInvalidAmount).
// Whether a code is a currency the rate provider knows is checked by the provider.
func NewConversion(from, to string, amount decimal.Decimal) (Conversion, error) {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))

	if !validCurrencyCode(from) || !validCurrencyCode(to) || from == to {
		return Conversion{}, ErrInvalidConversion
	}

	if !amount.IsPositive() || amount.Exponent() < -MaxAmountScale ||
		amount.Truncate(0).GreaterThanOrEqual(decimal.New(1, MaxAmountDigits)) {
		return Conversion{}, ErrInvalidAmount
	}

	return Conversion{From: from, To: to, Amount: amount}, nil
}

// ApplyRate sets a rate published on date: Result is Amount × Rate, rounded half-even.
func (c *Conversion) ApplyRate(rate decimal.Decimal, date time.Time) {
	c.Rate = rate
	c.RateDate = date
	c.Result = c.Amount.Mul(rate).RoundBank(_resultScale)
}

// ResultString is Result with every decimal place it is rounded to, as the API shows it: "0.30",
// not "0.3".
func (c *Conversion) ResultString() string {
	return c.Result.StringFixed(_resultScale)
}

// ReadableBy is nil when userID may read the conversion: when it is theirs. Another user's
// conversion is ErrConversionNotFound, the same as one that does not exist.
func (c *Conversion) ReadableBy(userID string) error {
	if c.UserID != userID {
		return ErrConversionNotFound
	}

	return nil
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
