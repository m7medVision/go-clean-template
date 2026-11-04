package query_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func readModel(t *testing.T) *repomock.MockExchangeReadModel {
	t.Helper()

	return repomock.NewMockExchangeReadModel(gomock.NewController(t))
}

func TestGetExchangeHistoryPagesTheUsersConversions(t *testing.T) {
	t.Parallel()

	read := readModel(t)
	want := []entity.Conversion{{From: "OMR", To: "CNY"}}
	read.EXPECT().History(gomock.Any(), "u1", int64(5), int64(10)).Return(want, 11, nil)

	got, err := query.GetExchangeHistoryHandler{History: read}.Handle(context.Background(),
		query.GetExchangeHistory{UserID: "u1", Limit: 5, Offset: 10})

	require.NoError(t, err)
	assert.Equal(t, entity.ExchangeHistory{History: want, Total: 11}, got)
}

func TestGetExchangeHistoryDefaultsThePage(t *testing.T) {
	t.Parallel()

	read := readModel(t)
	read.EXPECT().History(gomock.Any(), "u1", int64(10), int64(0)).Return(nil, 0, nil)

	_, err := query.GetExchangeHistoryHandler{History: read}.Handle(context.Background(),
		query.GetExchangeHistory{UserID: "u1", Limit: 0, Offset: -1})

	require.NoError(t, err)
}

func TestGetConversionReadsOneOfTheUsersConversions(t *testing.T) {
	t.Parallel()

	read := readModel(t)
	want := entity.Conversion{From: "OMR", To: "CNY"}
	read.EXPECT().Conversion(gomock.Any(), "u1", "c1").Return(want, nil)

	got, err := query.GetConversionHandler{History: read}.Handle(context.Background(),
		query.GetConversion{UserID: "u1", ConversionID: "c1"})

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetConversionKeepsNotFound(t *testing.T) {
	t.Parallel()

	read := readModel(t)
	read.EXPECT().Conversion(gomock.Any(), "u1", "c1").Return(entity.Conversion{}, entity.ErrConversionNotFound)

	_, err := query.GetConversionHandler{History: read}.Handle(context.Background(),
		query.GetConversion{UserID: "u1", ConversionID: "c1"})

	require.ErrorIs(t, err, entity.ErrConversionNotFound)
}
