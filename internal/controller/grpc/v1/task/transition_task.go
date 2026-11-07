package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TransitionTask -.
func (c *Controller) TransitionTask(ctx context.Context, req *v1.TransitionTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	_, err := cqrs.SendCommand(ctx, c.cmds, command.TransitionTask{
		UserID: userID, TaskID: req.GetId(), Status: entity.TaskStatus(req.GetStatus()),
	})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "TransitionTask")
	}

	resp, err := c.readBack(ctx, userID, req.GetId())
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "TransitionTask")
	}

	return resp, nil
}
