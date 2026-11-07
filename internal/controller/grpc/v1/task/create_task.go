package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// CreateTask -.
func (c *Controller) CreateTask(ctx context.Context, req *v1.CreateTaskRequest) (*v1.TaskResponse, error) {
	userID := authn.MustCaller(ctx).UserID

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
