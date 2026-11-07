package repo

import (
	"context"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/shopspring/decimal"
)

//go:generate mockgen -source=exchange.go -destination=repomock/exchange.go -package=repomock

type (
	// ExchangeReadModel reads exchange_history, which only the exchange projector writes. History is
	// one page of the user's conversions, newest first, and their total count. Conversion of a
	// missing conversion, or of an ID that is not a UUID, is entity.ErrConversionNotFound; whether
	// the caller may read it is entity.Conversion's rule.
	ExchangeReadModel interface {
		History(ctx context.Context, userID string, page entity.Page) (entity.List[entity.Conversion], error)
		Conversion(ctx context.Context, conversionID string) (entity.Conversion, error)
	}

	// ExchangeWebAPI fetches the reference rate from one currency to another, and the date it
	// was published. An unknown currency is entity.ErrUnsupportedCurrency, and a provider that
	// cannot be reached or fails entity.ErrRatesUnavailable.
	ExchangeWebAPI interface {
		Rate(ctx context.Context, from, to string) (decimal.Decimal, time.Time, error)
	}
)
