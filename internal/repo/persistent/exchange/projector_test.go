package exchange_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/exchange"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errWrite = errors.New("write failed")

// fakeQuerier records the conversion writes; any other query panics on the nil embedded interface.
type fakeQuerier struct {
	queries.Querier

	inserted []queries.InsertConversionRowParams
	err      error
}

//nolint:gocritic // the signature is sqlc's Querier, which takes its params by value
func (f *fakeQuerier) InsertConversionRow(_ context.Context, arg queries.InsertConversionRowParams) error {
	f.inserted = append(f.inserted, arg)

	return f.err
}

func at() time.Time { return time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC) }

func rateDate() time.Time { return time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC) }

func project(t *testing.T, q *fakeQuerier, e entity.Event) error {
	t.Helper()

	return exchange.Project(context.Background(), q, &entity.RecordedEvent{Position: 1, StreamID: "conversion-c1", Event: e})
}

func row(createdAt time.Time) queries.InsertConversionRowParams {
	return queries.InsertConversionRowParams{
		ConversionID: new("c1"), UserID: "u1", FromCurrency: "OMR", ToCurrency: "CNY",
		Amount: decimal.RequireFromString("100"), Rate: decimal.RequireFromString("17.4"),
		Result: decimal.RequireFromString("1740.00"), RateDate: rateDate(), CreatedAt: createdAt,
	}
}

func TestCurrencyConvertedInsertsAHistoryRow(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}

	require.NoError(t, project(t, q, entity.CurrencyConverted{
		ConversionID: "c1", UserID: "u1", From: "OMR", To: "CNY", Amount: decimal.RequireFromString("100"),
		Rate: decimal.RequireFromString("17.4"), Result: decimal.RequireFromString("1740.00"), RateDate: rateDate(), At: at(),
	}))

	assert.Equal(t, []queries.InsertConversionRowParams{row(at())}, q.inserted)
}

func TestConversionImportedInsertsTheRowItCarries(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}
	created := at().Add(-time.Hour)

	require.NoError(t, project(t, q, entity.ConversionImported{
		ConversionID: "c1", UserID: "u1", From: "OMR", To: "CNY", Amount: decimal.RequireFromString("100"),
		Rate: decimal.RequireFromString("17.4"), Result: decimal.RequireFromString("1740.00"), RateDate: rateDate(),
		CreatedAt: created,
	}))

	assert.Equal(t, []queries.InsertConversionRowParams{row(created)}, q.inserted)
}

func TestTheExchangeProjectorReturnsAWriteFailure(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, project(t, &fakeQuerier{err: errWrite}, entity.CurrencyConverted{ConversionID: "c1"}), errWrite)
}

func TestTheExchangeProjectorIgnoresOtherEvents(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}

	require.NoError(t, project(t, q, entity.TaskDeleted{TaskID: "t1"}))
	assert.Empty(t, q.inserted)
}

func TestTheExchangeProjectorOwnsTheHistoryTable(t *testing.T) {
	t.Parallel()

	p := exchange.NewProjector(nil)

	assert.Equal(t, "exchange", p.Name())
	assert.Equal(t, []string{"exchange_history"}, p.Tables())
}
