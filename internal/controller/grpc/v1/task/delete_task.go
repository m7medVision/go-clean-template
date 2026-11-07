package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (c *Controller) DeleteTask(ctx context.Context, req *v1.DeleteTaskRequest) (*v1.DeleteTaskResponse, error) {
	userID := authn.MustCaller(ctx).UserID

	_, err := cqrs.SendCommand(ctx, c.cmds, command.DeleteTask{UserID: userID, TaskID: req.GetId()})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "DeleteTask")
	}

	return &v1.DeleteTaskResponse{}, nil
}
