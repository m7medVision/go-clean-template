package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/internal/usecase/exchange"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newExchangeUseCase(t *testing.T) (usecase.Exchange, *MockExchangeRepo, *MockExchangeWebAPI) {
	t.Helper()

	ctrl := gomock.NewController(t)

	repo := NewMockExchangeRepo(ctrl)
	webAPI := NewMockExchangeWebAPI(ctrl)

	return exchange.New(repo, webAPI), repo, webAPI
}

func TestExchangeConvert(t *testing.T) {
	t.Parallel()

	rateDate := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	t.Run("converts at the provider's rate and stores it", func(t *testing.T) {
		t.Parallel()

		uc, repo, webAPI := newExchangeUseCase(t)
		webAPI.EXPECT().Rate(gomock.Any(), "EUR", "USD").Return(decimal.RequireFromString("1.1225"), rateDate, nil)
		repo.EXPECT().Store(gomock.Any(), "user-1", gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, c *entity.Conversion) error {
				assert.Equal(t, "112.36", c.Result.StringFixed(2))

				return nil
			})

		c, err := uc.Convert(context.Background(), "user-1", "eur", "usd", decimal.RequireFromString("100.10"))

		require.NoError(t, err)
		assert.Equal(t, "EUR", c.From)
		assert.Equal(t, "USD", c.To)
		assert.Equal(t, "112.36", c.Result.StringFixed(2))
		assert.Equal(t, rateDate, c.RateDate)
	})

	t.Run("invalid input never calls the provider", func(t *testing.T) {
		t.Parallel()

		uc, _, _ := newExchangeUseCase(t)

		_, err := uc.Convert(context.Background(), "user-1", "EUR", "EUR", decimal.RequireFromString("1"))

		require.ErrorIs(t, err, entity.ErrInvalidConversion)
	})

	t.Run("unsupported currency is passed through", func(t *testing.T) {
		t.Parallel()

		uc, _, webAPI := newExchangeUseCase(t)
		webAPI.EXPECT().Rate(gomock.Any(), "EUR", "XXX").Return(decimal.Decimal{}, time.Time{}, entity.ErrUnsupportedCurrency)

		_, err := uc.Convert(context.Background(), "user-1", "EUR", "XXX", decimal.RequireFromString("1"))

		require.ErrorIs(t, err, entity.ErrUnsupportedCurrency)
	})

	t.Run("store failure", func(t *testing.T) {
		t.Parallel()

		uc, repo, webAPI := newExchangeUseCase(t)
		webAPI.EXPECT().Rate(gomock.Any(), "EUR", "USD").Return(decimal.RequireFromString("1.1225"), rateDate, nil)
		repo.EXPECT().Store(gomock.Any(), "user-1", gomock.Any()).Return(errInternalServErr)

		_, err := uc.Convert(context.Background(), "user-1", "EUR", "USD", decimal.RequireFromString("1"))

		require.ErrorIs(t, err, errInternalServErr)
	})
}

func TestExchangeHistory(t *testing.T) {
	t.Parallel()

	t.Run("returns the stored conversions", func(t *testing.T) {
		t.Parallel()

		uc, repo, _ := newExchangeUseCase(t)
		stored := []entity.Conversion{{From: "EUR", To: "USD"}}
		repo.EXPECT().GetHistory(gomock.Any(), "user-1").Return(stored, nil)

		h, err := uc.History(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, entity.ExchangeHistory{History: stored}, h)
	})

	t.Run("repo failure", func(t *testing.T) {
		t.Parallel()

		uc, repo, _ := newExchangeUseCase(t)
		repo.EXPECT().GetHistory(gomock.Any(), "user-1").Return(nil, errInternalServErr)

		h, err := uc.History(context.Background(), "user-1")

		require.ErrorIs(t, err, errInternalServErr)
		assert.Equal(t, entity.ExchangeHistory{}, h)
	})
}
