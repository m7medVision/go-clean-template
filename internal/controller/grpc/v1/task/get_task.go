package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetTask -.
func (c *Controller) GetTask(ctx context.Context, req *v1.GetTaskRequest) (*v1.TaskResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	resp, err := c.readBack(ctx, userID, req.GetId())
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "GetTask")
	}

	return resp, nil
}
