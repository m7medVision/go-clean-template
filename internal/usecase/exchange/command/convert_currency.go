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
)

// ConvertCurrency converts Amount, a decimal string, from one currency to another at today's
// reference rate and records it in UserID's history. An invalid pair of codes is
// entity.ErrInvalidConversion, an amount that does not parse or fit entity.ErrInvalidAmount, and a
// currency the rate provider does not know entity.ErrUnsupportedCurrency.
type ConvertCurrency struct {
	cqrs.Command[entity.Written]

	UserID string
	From   string
	To     string
	Amount string
}

func (c ConvertCurrency) ActorID() string { return c.UserID }

// ConvertCurrencyHandler's Now is the clock (time.Now when nil).
type ConvertCurrencyHandler struct {
	Store repo.EventStore
	Rates repo.ExchangeWebAPI
	Now   func() time.Time
}

func (h ConvertCurrencyHandler) Handle(ctx context.Context, cmd ConvertCurrency) (entity.Written, error) {
	conversion, err := entity.NewConversion(cmd.From, cmd.To, cmd.Amount)
	if err != nil {
		return entity.Written{}, fmt.Errorf("ConvertCurrency - entity.NewConversion: %w", err)
	}

	rate, date, err := h.Rates.Rate(ctx, conversion.From, conversion.To)
	if err != nil {
		return entity.Written{}, fmt.Errorf("ConvertCurrency - h.Rates.Rate: %w", err)
	}

	conversion.ApplyRate(rate, date)

	id := uuid.NewString()

	written, err := aggregate.Save(ctx, h.Store,
		entity.NewConversionAggregate(id, cmd.UserID, &conversion, aggregate.Now(h.Now)))
	if err != nil {
		return entity.Written{}, fmt.Errorf("ConvertCurrency - aggregate.Save: %w", err)
	}

	return written, nil
}
