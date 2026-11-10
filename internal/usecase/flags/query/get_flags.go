// Package query holds the flags domain's query: the flags as the caller sees them.
package query

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/flags"
)

// GetFlags reads every flag for UserID, with the values the handlers that check them use.
type GetFlags struct {
	cqrs.Query[entity.FlagValues]

	UserID string
}

type GetFlagsHandler struct {
	Flags *flags.Client
}

func (h GetFlagsHandler) Handle(ctx context.Context, q GetFlags) (entity.FlagValues, error) {
	f := entity.FlagTaskAttributes()

	return entity.FlagValues{TaskAttributes: h.Flags.Bool(ctx, f.Key, f.Default, q.UserID)}, nil
}
