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

// CreateTask -.
func (c *Controller) CreateTask(ctx context.Context, req *v1.CreateTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	written, err := cqrs.SendCommand(ctx, c.cmds, command.CreateTask{
		UserID: userID, Title: req.GetTitle(), Description: req.GetDescription(),
	})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "CreateTask")
	}

	resp, err := c.readBack(ctx, userID, written.ID)
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "CreateTask")
	}

	return resp, nil
}
