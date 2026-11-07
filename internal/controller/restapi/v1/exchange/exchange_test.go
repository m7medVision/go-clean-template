package exchange_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/controller/restapi/resttest"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertExchange(t *testing.T) {
	t.Parallel()

	token := resttest.Token(t)
	body := `{"from":"eur","to":"USD","amount":"100.10"}`
	converted := entity.Conversion{
		From: "EUR", To: "USD",
		Amount:   decimal.RequireFromString("100.10"),
		Rate:     decimal.RequireFromString("1.1225"),
		Result:   decimal.RequireFromString("112.36"),
		RateDate: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
	}

	t.Run("success returns amounts as strings and the rate date", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(&fakeExchange{converted: converted})

		resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/exchange/convert", token, body)
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		assert.JSONEq(t,
			`{"from":"EUR","to":"USD","amount":"100.1","rate":"1.1225","result":"112.36","rate_date":"2026-10-19"}`,
			resp.Body.String())
	})

	t.Run("malformed amount is rejected by validation", func(t *testing.T) {
		t.Parallel()

		handler := newRouter(&fakeExchange{converted: converted})

		resp := resttest.Do(t.Context(), handler, http.MethodPost, "/v1/exchange/convert", token,
			`{"from":"EUR","to":"USD","amount":"1e3"}`)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	})
}

func TestExchangeHistory(t *testing.T) {
	t.Parallel()

	exchange := &fakeExchange{converted: entity.Conversion{
		From: "EUR", To: "USD", Amount: decimal.RequireFromString("1"),
		Rate: decimal.RequireFromString("1.1225"), Result: decimal.RequireFromString("1.12"),
		RateDate: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
	}}
	handler := newRouter(exchange)

	resp := resttest.Do(t.Context(), handler, http.MethodGet, "/v1/exchange/history?limit=5&offset=10", resttest.Token(t), "")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.JSONEq(t,
		`{"history":[{"from":"EUR","to":"USD","amount":"1","rate":"1.1225","result":"1.12","rate_date":"2026-10-19"}],"total":11}`,
		resp.Body.String())
	assert.Equal(t, 5, exchange.gotLimit)
	assert.Equal(t, 10, exchange.gotOffset)
}
