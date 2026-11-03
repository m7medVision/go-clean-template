package exchange_test

import (
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/exchange"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConversionResponse(t *testing.T) {
	t.Parallel()

	c := entity.Conversion{
		From:     "EUR",
		To:       "USD",
		Amount:   decimal.RequireFromString("100.10"),
		Rate:     decimal.RequireFromString("1.1225"),
		Result:   decimal.RequireFromString("112.36"),
		RateDate: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
	}

	resp := exchange.NewConversionResponse(&c)

	require.NotNil(t, resp)
	assert.Equal(t, "EUR", resp.GetFrom())
	assert.Equal(t, "USD", resp.GetTo())
	assert.Equal(t, "100.1", resp.GetAmount())
	assert.Equal(t, "1.1225", resp.GetRate())
	assert.Equal(t, "112.36", resp.GetResult())
	assert.Equal(t, "2026-10-19", resp.GetRateDate())
}

func TestNewExchangeHistoryResponse(t *testing.T) {
	t.Parallel()

	history := entity.ExchangeHistory{History: []entity.Conversion{
		{From: "EUR", To: "USD", Result: decimal.RequireFromString("1")},
		{From: "GBP", To: "JPY", Result: decimal.RequireFromString("2.5")},
	}, Total: 7}

	resp := exchange.NewExchangeHistoryResponse(history)

	require.Len(t, resp.GetHistory(), 2)
	assert.Equal(t, "EUR", resp.GetHistory()[0].GetFrom())
	assert.Equal(t, "1.00", resp.GetHistory()[0].GetResult())
	assert.Equal(t, "JPY", resp.GetHistory()[1].GetTo())
	assert.Equal(t, "2.50", resp.GetHistory()[1].GetResult())
	assert.Equal(t, int32(7), resp.GetTotal())
}
