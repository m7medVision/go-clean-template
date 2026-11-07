package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UpdateTask -.
func (c *Controller) UpdateTask(ctx context.Context, req *v1.UpdateTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	_, err := cqrs.SendCommand(ctx, c.cmds, command.UpdateTask{
		UserID: userID, TaskID: req.GetId(), Title: req.GetTitle(), Description: req.GetDescription(),
	})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "UpdateTask")
	}

	resp, err := c.readBack(ctx, userID, req.GetId())
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "UpdateTask")
	}

	return resp, nil
}
