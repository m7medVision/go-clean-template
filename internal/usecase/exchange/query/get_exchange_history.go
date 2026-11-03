// Package query holds the exchange domain's queries, one file each, with their handlers. They read
// only the exchange read model.
package query

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

// _defaultLimit is the page size GetExchangeHistory uses when the caller asks for none.
const _defaultLimit = 10

// GetExchangeHistory reads one page of UserID's conversions, newest first. A Limit of 0 or less
// is the default page; a negative Offset is 0.
type GetExchangeHistory struct {
	UserID string
	Limit  int
	Offset int
}

// QueryName -.
func (GetExchangeHistory) QueryName() string { return "GetExchangeHistory" }

// GetExchangeHistoryHandler handles GetExchangeHistory.
type GetExchangeHistoryHandler struct {
	History repo.ExchangeReadModel
}

// Handle -.
func (h GetExchangeHistoryHandler) Handle(ctx context.Context, q GetExchangeHistory) (entity.ExchangeHistory, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = _defaultLimit
	}

	conversions, total, err := h.History.History(ctx, q.UserID, int64(limit), int64(max(q.Offset, 0)))
	if err != nil {
		return entity.ExchangeHistory{}, fmt.Errorf("GetExchangeHistory - h.History.History: %w", err)
	}

	return entity.ExchangeHistory{History: conversions, Total: total}, nil
}
