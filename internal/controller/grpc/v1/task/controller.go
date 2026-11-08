// Package task serves TaskService (docs/proto/v1/task.proto) over gRPC.
package task

import (
	"context"
	"log/slog"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type Controller struct {
	v1.UnimplementedTaskServiceServer

	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

func New(b cqrs.Buses, l *slog.Logger) v1.TaskServiceServer {
	return &Controller{cmds: b.Commands, qs: b.Queries, l: l}
}

func (c *Controller) readBack(ctx context.Context, userID, taskID string) (*v1.TaskResponse, error) {
	task, err := cqrs.SendQuery(ctx, c.qs, query.GetTask{UserID: userID, TaskID: taskID})
	if err != nil {
		return nil, err
	}

	return NewTaskResponse(&task), nil
}

func listPage[Q cqrs.QueryOf[entity.List[mongotask.TaskWithAttributes]]](ctx context.Context, c *Controller, q Q, op string) (*v1.ListTasksResponse, error) {
	page, err := cqrs.SendQuery(ctx, c.qs, q)
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, op)
	}

	return NewListTasksResponse(page.Items, page.Total), nil
}
