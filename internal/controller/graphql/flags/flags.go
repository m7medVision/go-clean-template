package flags

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/flags/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (q Query) Flags(ctx context.Context) (*entity.FlagValues, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	values, err := cqrs.SendQuery(ctx, q.qs, query.GetFlags{UserID: caller.UserID})
	if err != nil {
		return nil, gqlerr.Map(ctx, q.l, err, "flags")
	}

	return &values, nil
}
