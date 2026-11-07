package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (q Query) Task(ctx context.Context, id string) (*entity.Task, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	task, err := cqrs.SendQuery(ctx, q.qs, query.GetTask{UserID: caller.UserID, TaskID: id})
	if err != nil {
		return nil, gqlerr.Map(ctx, q.l, err, "task")
	}

	return &task, nil
}
