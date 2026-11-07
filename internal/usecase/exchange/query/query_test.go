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
