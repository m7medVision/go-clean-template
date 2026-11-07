// Package query holds the exchange domain's queries, one file each, with their handlers. They read
// only the exchange read model.
package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// GetExchangeHistory reads one page of UserID's conversions, newest first.
type GetExchangeHistory struct {
	cqrs.Query[entity.List[entity.Conversion]]
	entity.Page

	UserID string
}

type GetExchangeHistoryHandler struct {
	History repo.ExchangeReadModel
}

func (h GetExchangeHistoryHandler) Handle(ctx context.Context, q GetExchangeHistory) (entity.List[entity.Conversion], error) {
	history, err := h.History.History(ctx, q.UserID, q.Page)
	if err != nil {
		return entity.List[entity.Conversion]{}, fmt.Errorf("GetExchangeHistory - h.History.History: %w", err)
	}

	return history, nil
}
