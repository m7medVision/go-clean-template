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
		wantErr  bool
		wantFrom string
		wantTo   string
	}{
		{"valid", "EUR", "USD", "100", false, "EUR", "USD"},
		{"lowercase and spaces are normalized", " eur ", "usd", "1.5", false, "EUR", "USD"},
		{"same currency", "EUR", "eur", "1", true, "", ""},
		{"code too short", "EU", "USD", "1", true, "", ""},
		{"code too long", "EURO", "USD", "1", true, "", ""},
		{"code with digits", "EU1", "USD", "1", true, "", ""},
		{"zero amount", "EUR", "USD", "0", true, "", ""},
		{"negative amount", "EUR", "USD", "-5", true, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := entity.NewConversion(tt.from, tt.to, decimal.RequireFromString(tt.amount))

			if tt.wantErr {
				require.ErrorIs(t, err, entity.ErrInvalidConversion)

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

			assert.Equal(t, tt.wantResult, c.Result.StringFixed(2))
			assert.True(t, c.Rate.Equal(decimal.RequireFromString(tt.rate)))
			assert.Equal(t, date, c.RateDate)
		})
	}
}
