// Package command holds the exchange domain's commands, one file each, with their handlers.
package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/usecase/aggregate"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/shopspring/decimal"
)

// ConvertCurrency converts Amount from one currency to another at today's reference rate and
// records it in UserID's history. An invalid request is entity.ErrInvalidConversion, and a currency
// the rate provider does not know entity.ErrUnsupportedCurrency.
type ConvertCurrency struct {
	cqrs.Command[cqrs.Written]

	UserID string
	From   string
	To     string
	Amount decimal.Decimal
}

// ActorID -.
func (c ConvertCurrency) ActorID() string { return c.UserID }

// ConvertCurrencyHandler handles ConvertCurrency. Now is the clock (time.Now when nil).
type ConvertCurrencyHandler struct {
	Store repo.EventStore
	Rates repo.ExchangeWebAPI
	Now   func() time.Time
}

// Handle -.
func (h ConvertCurrencyHandler) Handle(ctx context.Context, cmd ConvertCurrency) (cqrs.Written, error) {
	conversion, err := entity.NewConversion(cmd.From, cmd.To, cmd.Amount)
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("ConvertCurrency - entity.NewConversion: %w", err)
	}

	rate, date, err := h.Rates.Rate(ctx, conversion.From, conversion.To)
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("ConvertCurrency - h.Rates.Rate: %w", err)
	}

	conversion.ApplyRate(rate, date)

	id := uuid.NewString()

	written, err := aggregate.Create(ctx, h.Store, id,
		entity.NewConversionAggregate(id, cmd.UserID, &conversion, aggregate.Now(h.Now)))
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("ConvertCurrency - aggregate.Create: %w", err)
	}

	return written, nil
}
