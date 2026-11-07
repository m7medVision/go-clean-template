package entity_test

import (
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConversion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		from, to string
		amount   string
		wantErr  error
		wantFrom string
		wantTo   string
	}{
		{"valid", "EUR", "USD", "100", nil, "EUR", "USD"},
		{"lowercase and spaces are normalized", " eur ", "usd", "1.5", nil, "EUR", "USD"},
		{"the largest amount", "EUR", "USD", "999999999999999.12345678", nil, "EUR", "USD"},
		{"an exponent within bounds", "EUR", "USD", "1e3", nil, "EUR", "USD"},
		{"same currency", "EUR", "eur", "1", entity.ErrInvalidConversion, "", ""},
		{"code too short", "EU", "USD", "1", entity.ErrInvalidConversion, "", ""},
		{"code too long", "EURO", "USD", "1", entity.ErrInvalidConversion, "", ""},
		{"code with digits", "EU1", "USD", "1", entity.ErrInvalidConversion, "", ""},
		{"zero amount", "EUR", "USD", "0", entity.ErrInvalidAmount, "", ""},
		{"negative amount", "EUR", "USD", "-5", entity.ErrInvalidAmount, "", ""},
		{"16 digits before the point", "EUR", "USD", "1000000000000000", entity.ErrInvalidAmount, "", ""},
		{"9 digits after the point", "EUR", "USD", "1.123456789", entity.ErrInvalidAmount, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := entity.NewConversion(tt.from, tt.to, decimal.RequireFromString(tt.amount))

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantFrom, c.From)
			assert.Equal(t, tt.wantTo, c.To)
			assert.True(t, c.Amount.Equal(decimal.RequireFromString(tt.amount)))
		})
	}
}

func TestConversion_ApplyRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		amount     string
		rate       string
		wantResult string
	}{
		{"exact decimal multiplication", "100.10", "1.1225", "112.36"},
		{"no float drift", "0.1", "3", "0.30"},
		{"half-even rounds down on an even digit", "1", "0.125", "0.12"},
		{"half-even rounds up on an odd digit", "1", "0.135", "0.14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := entity.NewConversion("EUR", "USD", decimal.RequireFromString(tt.amount))
			require.NoError(t, err)

			date := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
			c.ApplyRate(decimal.RequireFromString(tt.rate), date)

			assert.Equal(t, tt.wantResult, c.ResultString())
			assert.True(t, c.Rate.Equal(decimal.RequireFromString(tt.rate)))
			assert.Equal(t, date, c.RateDate)
		})
	}
}
