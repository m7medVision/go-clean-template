package entity_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _conversionID = "conv-1"

func converted(t *testing.T) entity.Conversion {
	t.Helper()

	c, err := entity.NewConversion("omr", "cny", "100")
	require.NoError(t, err)
	c.ApplyRate(decimal.RequireFromString("17.406483"), time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC))

	return c
}

func TestNewConversionAggregateRecordsCurrencyConverted(t *testing.T) {
	t.Parallel()

	c := converted(t)

	a := entity.NewConversionAggregate(_conversionID, _owner, &c, createdAt())

	assert.Equal(t, []entity.Event{entity.CurrencyConverted{
		ConversionID: _conversionID, UserID: _owner, From: "OMR", To: "CNY",
		Amount: c.Amount, Rate: c.Rate, Result: c.Result, RateDate: c.RateDate, At: createdAt(),
	}}, a.Pending())
	assert.Equal(t, "conversion-"+_conversionID, a.StreamID())
}

func TestCurrencyConvertedKeepsAmountsExactThroughJSON(t *testing.T) {
	t.Parallel()

	c := converted(t)
	event := entity.CurrencyConverted{ConversionID: _conversionID, Amount: c.Amount, Rate: c.Rate, Result: c.Result}

	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"rate":"17.406483"`, "a decimal is stored as a string, never a float")

	var decoded entity.CurrencyConverted
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.True(t, decoded.Result.Equal(c.Result))
}

func TestImportConversionAggregateRecordsTheRow(t *testing.T) {
	t.Parallel()

	c := converted(t)

	a := entity.ImportConversionAggregate(_conversionID, _owner, &c, createdAt())

	assert.Equal(t, []entity.Event{entity.ConversionImported{
		ConversionID: _conversionID, UserID: _owner, From: "OMR", To: "CNY",
		Amount: c.Amount, Rate: c.Rate, Result: c.Result, RateDate: c.RateDate, CreatedAt: createdAt(),
	}}, a.Pending())
}
