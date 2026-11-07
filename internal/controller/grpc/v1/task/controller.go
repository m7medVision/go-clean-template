// Package task serves TaskService (docs/proto/v1/task.proto) over gRPC.
package task

import (
	"context"
	"log/slog"

	"github.com/go-playground/validator/v10"
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Controller -.
type Controller struct {
	v1.UnimplementedTaskServiceServer

	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
	v    *validator.Validate
}

// New -.
func New(b cqrs.Buses, l *slog.Logger) v1.TaskServiceServer {
	return &Controller{cmds: b.Commands, qs: b.Queries, l: l, v: validator.New(validator.WithRequiredStructEnabled())}
}

// readBack reads the task a command just wrote, for the response.
func (c *Controller) readBack(ctx context.Context, userID, taskID string) (*v1.TaskResponse, error) {
	task, err := cqrs.SendQuery[entity.Task](ctx, c.qs, query.GetTask{UserID: userID, TaskID: taskID})
	if err != nil {
		return nil, err
	}

	return NewTaskResponse(&task), nil
}
