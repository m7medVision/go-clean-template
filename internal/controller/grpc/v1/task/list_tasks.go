package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListTasks -.
func (c *Controller) ListTasks(ctx context.Context, req *v1.ListTasksRequest) (*v1.ListTasksResponse, error) {
	userID, ok := grpcmw.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	return listPage(ctx, c, query.ListTasks{
		UserID: userID, Status: listStatus(req), Limit: int(req.GetLimit()), Offset: int(req.GetOffset()),
	}, "ListTasks")
}
