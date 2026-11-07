package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (c *Controller) UpdateTask(ctx context.Context, req *v1.UpdateTaskRequest) (*v1.TaskResponse, error) {
	userID := authn.MustCaller(ctx).UserID

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
