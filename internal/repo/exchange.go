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
	// was published. A currency the provider does not know is entity.ErrUnsupportedCurrency. Any
	// other failure of the provider (unreachable, timed out, an error status or result, a body that
	// does not decode) is entity.ErrRatesUnavailable; a canceled ctx is neither.
	ExchangeWebAPI interface {
		Rate(ctx context.Context, from, to string) (decimal.Decimal, time.Time, error)
	}
)
