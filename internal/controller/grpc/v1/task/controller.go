// Package task serves TaskService (docs/proto/v1/task.proto) over gRPC.
package task

import (
	"context"
	"errors"

	"github.com/go-playground/validator/v10"
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Controller -.
type Controller struct {
	v1.UnimplementedTaskServiceServer

	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    logger.Interface
	v    *validator.Validate
}

// New -.
func New(b cqrs.Buses, l logger.Interface) v1.TaskServiceServer {
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

// fail maps a failed command or query to its status: first what any command can fail with, then
// the task errors, and anything else to a logged Internal.
func (c *Controller) fail(err error, op string) error {
	if shared := rpcerr.Shared(err); shared != nil {
		c.l.Warn("grpc - v1 - %s: %v", op, err)

		return shared
	}

	switch {
	case errors.Is(err, entity.ErrTaskNotFound):
		return status.Error(codes.NotFound, "task not found")
	case errors.Is(err, entity.ErrTaskForbidden):
		return status.Error(codes.PermissionDenied, "forbidden")
	case errors.Is(err, entity.ErrAdminRequired):
		return status.Error(codes.PermissionDenied, "admin role required")
	case errors.Is(err, entity.ErrInvalidTransition):
		return status.Error(codes.InvalidArgument, "invalid status transition")
	default:
		c.l.Error(err, "grpc - v1 - "+op)

		return status.Error(codes.Internal, "internal server error")
	}
}
