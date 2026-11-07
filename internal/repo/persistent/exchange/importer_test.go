package exchange_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/exchange"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rows is an exchange_history table holding legacy rows, recording the conversion IDs the importer
// gives them; any other query panics on the nil embedded interface.
type rows struct {
	queries.Querier

	history []queries.ExchangeHistory
	given   []queries.SetConversionIDParams
	err     error
}

func (r *rows) ListEveryExchange(context.Context) ([]queries.ExchangeHistory, error) {
	return r.history, r.err
}

func (r *rows) SetConversionID(_ context.Context, arg queries.SetConversionIDParams) error {
	r.given = append(r.given, arg)

	return nil
}

func legacy(id int64, conversionID *string) queries.ExchangeHistory {
	return queries.ExchangeHistory{
		ID: id, ConversionID: conversionID, UserID: "u1", FromCurrency: "OMR", ToCurrency: "CNY",
		Amount: decimal.RequireFromString("100"), Rate: decimal.RequireFromString("17.4"),
		Result: decimal.RequireFromString("1740.00"), RateDate: rateDate(), CreatedAt: at(),
	}
}

func imported(t *testing.T, a entity.Aggregate) entity.ConversionImported {
	t.Helper()

	require.Len(t, a.Pending(), 1)

	e, ok := a.Pending()[0].(entity.ConversionImported)
	require.True(t, ok)

	return e
}

func TestEachExchangeRowBecomesAStreamHoldingConversionImported(t *testing.T) {
	t.Parallel()

	c1 := "c1"
	q := &rows{history: []queries.ExchangeHistory{legacy(1, &c1)}}

	imports, err := exchange.Imports(context.Background(), q)
	require.NoError(t, err)
	require.Len(t, imports, 1)

	assert.Equal(t, entity.ConversionStreamID("c1"), imports[0].Aggregate.StreamID())

	e := imported(t, imports[0].Aggregate)
	assert.Equal(t, "c1", e.ConversionID)
	assert.Equal(t, "u1", e.UserID)
	assert.Equal(t, at(), e.CreatedAt)
	assert.True(t, decimal.RequireFromString("1740.00").Equal(e.Result))
	assert.Empty(t, q.given, "a row that has its conversion ID keeps it")
}

func TestARowWithoutAConversionIDGetsOneItsStreamShares(t *testing.T) {
	t.Parallel()

	q := &rows{history: []queries.ExchangeHistory{legacy(7, nil)}}

	imports, err := exchange.Imports(context.Background(), q)
	require.NoError(t, err)
	require.Len(t, q.given, 1)

	assert.Equal(t, int64(7), q.given[0].ID)
	assert.Equal(t, *q.given[0].ConversionID, imported(t, imports[0].Aggregate).ConversionID)
}

func TestTheExchangeImporterReturnsAReadFailure(t *testing.T) {
	t.Parallel()

	_, err := exchange.Imports(context.Background(), &rows{err: errWrite})

	require.ErrorIs(t, err, errWrite)
}

func TestTheImporterIsTheExchangeDomains(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "exchange", exchange.NewImporter(nil).Domain())
}
