package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/command"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func rateDate() time.Time { return time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC) }

func now() time.Time { return time.Date(2026, 10, 19, 9, 0, 0, 999, time.UTC) }

func TestConvertCurrencyRecordsTheConversionAtTheRate(t *testing.T) {
	t.Parallel()

	rates := repomock.NewMockExchangeWebAPI(gomock.NewController(t))
	rates.EXPECT().Rate(gomock.Any(), "OMR", "CNY").Return(decimal.RequireFromString("17.406483"), rateDate(), nil)

	store := memstore.New(entity.EventTypes())
	h := command.ConvertCurrencyHandler{Store: store, Rates: rates, Now: now}

	written, err := h.Handle(context.Background(),
		command.ConvertCurrency{UserID: "u1", From: " omr", To: "cny", Amount: decimal.RequireFromString("100")})
	require.NoError(t, err)

	events := store.Events(entity.ConversionStreamID(written.ID))
	require.Len(t, events, 1)

	converted, ok := events[0].(entity.CurrencyConverted)
	require.True(t, ok)
	assert.Equal(t, "u1", converted.UserID)
	assert.Equal(t, "OMR", converted.From)
	assert.Equal(t, "CNY", converted.To)
	assert.Equal(t, "1740.65", converted.Result.String())
	assert.Equal(t, rateDate(), converted.RateDate)
	assert.Equal(t, now().Truncate(time.Microsecond), converted.At)
}

func TestConvertCurrencyRefusesAnInvalidRequestBeforeAskingForARate(t *testing.T) {
	t.Parallel()

	rates := repomock.NewMockExchangeWebAPI(gomock.NewController(t))
	h := command.ConvertCurrencyHandler{Store: memstore.New(entity.EventTypes()), Rates: rates}

	_, err := h.Handle(context.Background(),
		command.ConvertCurrency{UserID: "u1", From: "OMR", To: "OMR", Amount: decimal.RequireFromString("1")})

	require.ErrorIs(t, err, entity.ErrInvalidConversion)
}

func TestConvertCurrencyKeepsTheRateProvidersError(t *testing.T) {
	t.Parallel()

	rates := repomock.NewMockExchangeWebAPI(gomock.NewController(t))
	rates.EXPECT().Rate(gomock.Any(), "OMR", "XXX").Return(decimal.Decimal{}, time.Time{}, entity.ErrUnsupportedCurrency)

	h := command.ConvertCurrencyHandler{Store: memstore.New(entity.EventTypes()), Rates: rates}

	_, err := h.Handle(context.Background(),
		command.ConvertCurrency{UserID: "u1", From: "OMR", To: "XXX", Amount: decimal.RequireFromString("1")})

	require.ErrorIs(t, err, entity.ErrUnsupportedCurrency)
}
