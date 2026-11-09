// Package event holds the exchange domain's event handlers, one file each: what follows from an
// exchange event, outside the conversion's own state.
package event

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// CaptureExchange sends the exchange domain's analytics events: the currencies, never the amount.
// It starts at the store's head, so the conversions made before analytics was configured are never
// sent.
type CaptureExchange struct {
	Analytics repo.Analytics
}

var (
	_ repo.Subscriber   = CaptureExchange{}
	_ repo.StartsAtHead = CaptureExchange{}
)

func (CaptureExchange) Name() string { return "capture-exchange" }

func (CaptureExchange) StartsAtHead() bool { return true }

func (CaptureExchange) Handles() []entity.Event {
	return []entity.Event{entity.CurrencyConverted{}}
}

func (h CaptureExchange) Handle(ctx context.Context, e *entity.RecordedEvent) error {
	converted, ok := e.Event.(entity.CurrencyConverted)
	if !ok {
		return fmt.Errorf("CaptureExchange - %s: %w", e.Event.EventType(), entity.ErrUnhandledEvent)
	}

	err := h.Analytics.Capture(ctx, &analytics.Event{
		ID:         analytics.EventID(e),
		DistinctID: converted.UserID,
		At:         e.RecordedAt,
		Properties: analytics.CurrencyConverted{From: converted.From, To: converted.To},
	})
	if err != nil {
		return fmt.Errorf("CaptureExchange - h.Analytics.Capture: %w", err)
	}

	return nil
}
