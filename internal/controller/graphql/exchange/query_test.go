package exchange_test

import (
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _exchangeHistory = `query { exchangeHistory(limit: 5, offset: 10) {
	total conversions { from to amount rate result rateDate } } }`

func TestExchangeHistory(t *testing.T) {
	t.Parallel()

	ex := &fakeExchange{history: entity.List[entity.Conversion]{Total: 11, Items: []entity.Conversion{{
		From: "OMR", To: "CNY",
		Amount:   decimal.RequireFromString("100.10"),
		Rate:     decimal.RequireFromString("17.406483"),
		Result:   decimal.RequireFromString("1742.39"),
		RateDate: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
	}}}}

	resp := do(t, ex, true, _exchangeHistory)

	require.Empty(t, resp.Errors)
	assert.JSONEq(t, `{"total":11,"conversions":[{"from":"OMR","to":"CNY","amount":"100.1",`+
		`"rate":"17.406483","result":"1742.39","rateDate":"2026-10-19"}]}`, string(resp.Data["exchangeHistory"]))
	assert.Equal(t, 5, ex.gotLimit)
	assert.Equal(t, 10, ex.gotOffset)
}

func TestExchangeHistoryUnauthenticated(t *testing.T) {
	t.Parallel()

	ex := &fakeExchange{}

	resp := do(t, ex, false, _exchangeHistory)

	assert.Equal(t, "UNAUTHENTICATED", resp.Code(t))
	assert.False(t, ex.called)
}
