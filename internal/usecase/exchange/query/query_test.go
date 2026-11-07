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

func TestGetConversionReadsTheCallersConversionOnly(t *testing.T) {
	t.Parallel()

	want := entity.Conversion{UserID: "u1", From: "EUR", To: "USD"}
	history := repomock.NewMockExchangeReadModel(gomock.NewController(t))
	history.EXPECT().Conversion(gomock.Any(), "c1").Return(want, nil).Times(2)
	h := query.GetConversionHandler{History: history}

	got, err := h.Handle(context.Background(), query.GetConversion{UserID: "u1", ConversionID: "c1"})
	require.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = h.Handle(context.Background(), query.GetConversion{UserID: "u2", ConversionID: "c1"})
	require.ErrorIs(t, err, entity.ErrConversionNotFound, "another user's conversion, as one that does not exist")
}
