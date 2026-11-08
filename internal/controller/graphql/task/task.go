package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (q Query) Task(ctx context.Context, id string) (*mongotask.TaskWithAttributes, error) {
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
