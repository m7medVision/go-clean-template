package event_test

import (
	"errors"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/event"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errQueue = errors.New("queue down")

func TestCaptureExchangeSendsTheCurrenciesNotTheAmount(t *testing.T) {
	t.Parallel()

	at := time.Unix(1, 0).UTC()
	a := repomock.NewMockAnalytics(gomock.NewController(t))
	a.EXPECT().Capture(gomock.Any(), &analytics.Event{
		ID: "conversion-c1-1", DistinctID: "u1", At: at,
		Properties: analytics.CurrencyConverted{From: "USD", To: "EUR"},
	}).Return(nil)

	require.NoError(t, event.CaptureExchange{Analytics: a}.Handle(t.Context(), &entity.RecordedEvent{
		StreamID: "conversion-c1", StreamVersion: 1, RecordedAt: at,
		Event: entity.CurrencyConverted{ConversionID: "c1", UserID: "u1", From: "USD", To: "EUR", Amount: decimal.NewFromInt(10)},
	}))
}

func TestCaptureExchangeStartsAtHeadAndSkipsImports(t *testing.T) {
	t.Parallel()

	h := event.CaptureExchange{}
	assert.True(t, h.StartsAtHead())
	assert.NotContains(t, h.Handles(), entity.Event(entity.ConversionImported{}))
}

func TestCaptureExchangeReturnsAFailureForTheRunnerToRetry(t *testing.T) {
	t.Parallel()

	a := repomock.NewMockAnalytics(gomock.NewController(t))
	a.EXPECT().Capture(gomock.Any(), gomock.Any()).Return(errQueue)

	require.ErrorIs(t, event.CaptureExchange{Analytics: a}.Handle(t.Context(),
		&entity.RecordedEvent{StreamID: "conversion-c1", Event: entity.CurrencyConverted{}}), errQueue)
}
