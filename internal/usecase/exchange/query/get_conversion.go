package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// GetConversion reads one of UserID's conversions: what a transport answers ConvertCurrency with.
// A missing one, or another user's, is entity.ErrConversionNotFound.
type GetConversion struct {
	cqrs.Query[entity.Conversion]

	UserID       string
	ConversionID string
}

// GetConversionHandler handles GetConversion.
type GetConversionHandler struct {
	History repo.ExchangeReadModel
}

// Handle -.
func (h GetConversionHandler) Handle(ctx context.Context, q GetConversion) (entity.Conversion, error) {
	conversion, err := h.History.Conversion(ctx, q.UserID, q.ConversionID)
	if err != nil {
		return entity.Conversion{}, fmt.Errorf("GetConversion - h.History.Conversion: %w", err)
	}

	return conversion, nil
}
