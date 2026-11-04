package exchange_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/exchange"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func imported(id string) exchange.ImportedConversion {
	return exchange.ImportedConversion{
		ID: id, UserID: "u1", CreatedAt: at(),
		Conversion: entity.Conversion{
			From: "OMR", To: "CNY", Amount: decimal.RequireFromString("100"), Rate: decimal.RequireFromString("17.4"),
			Result: decimal.RequireFromString("1740.00"), RateDate: rateDate(),
		},
	}
}

func TestImportConversionsGivesEachRowItsStream(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())

	require.NoError(t, exchange.ImportConversions(context.Background(), store,
		[]exchange.ImportedConversion{imported("c1"), imported("c2")}))

	for _, id := range []string{"c1", "c2"} {
		events := store.Events(entity.ConversionStreamID(id))
		require.Len(t, events, 1)

		e, ok := events[0].(entity.ConversionImported)
		require.True(t, ok)
		assert.Equal(t, id, e.ConversionID)
		assert.Equal(t, "u1", e.UserID)
		assert.Equal(t, at(), e.CreatedAt)
	}
}

func TestImportConversionsSkipsAConversionThatHasAStreamAlready(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())
	c := imported("c1")

	require.NoError(t, exchange.ImportConversions(context.Background(), store, []exchange.ImportedConversion{c}))
	require.NoError(t, exchange.ImportConversions(context.Background(), store, []exchange.ImportedConversion{c}))

	assert.Len(t, store.Events(entity.ConversionStreamID("c1")), 1)
}

func TestTheImporterIsTheExchangeDomains(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "exchange", exchange.NewImporter(nil, nil).Domain())
}
