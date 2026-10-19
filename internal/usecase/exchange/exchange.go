// Package exchange implements currency conversion at reference rates.
package exchange

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/shopspring/decimal"
)

// UseCase -.
type UseCase struct {
	repo   repo.ExchangeRepo
	webAPI repo.ExchangeWebAPI
}

// New returns an Exchange usecase instrumented with OpenTelemetry tracing spans.
func New(r repo.ExchangeRepo, w repo.ExchangeWebAPI) usecase.Exchange {
	return newTraced(&UseCase{
		repo:   r,
		webAPI: w,
	})
}

// Convert validates the request, fetches the current rate, and stores the conversion.
func (uc *UseCase) Convert(
	ctx context.Context, userID, from, to string, amount decimal.Decimal,
) (entity.Conversion, error) {
	conversion, err := entity.NewConversion(from, to, amount)
	if err != nil {
		return entity.Conversion{}, fmt.Errorf("ExchangeUseCase - Convert - entity.NewConversion: %w", err)
	}

	rate, date, err := uc.webAPI.Rate(ctx, conversion.From, conversion.To)
	if err != nil {
		return entity.Conversion{}, fmt.Errorf("ExchangeUseCase - Convert - uc.webAPI.Rate: %w", err)
	}

	conversion.ApplyRate(rate, date)

	if err = uc.repo.Store(ctx, userID, &conversion); err != nil {
		return entity.Conversion{}, fmt.Errorf("ExchangeUseCase - Convert - uc.repo.Store: %w", err)
	}

	return conversion, nil
}

// History returns the user's conversions, newest first.
func (uc *UseCase) History(ctx context.Context, userID string) (entity.ExchangeHistory, error) {
	conversions, err := uc.repo.GetHistory(ctx, userID)
	if err != nil {
		return entity.ExchangeHistory{}, fmt.Errorf("ExchangeUseCase - History - uc.repo.GetHistory: %w", err)
	}

	return entity.ExchangeHistory{History: conversions}, nil
}
