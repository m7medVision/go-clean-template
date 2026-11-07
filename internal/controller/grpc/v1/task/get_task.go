package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
)

// GetTask -.
func (c *Controller) GetTask(ctx context.Context, req *v1.GetTaskRequest) (*v1.TaskResponse, error) {
	userID := authn.MustCaller(ctx).UserID

	resp, err := c.readBack(ctx, userID, req.GetId())
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "GetTask")
	}

	return resp, nil
}
