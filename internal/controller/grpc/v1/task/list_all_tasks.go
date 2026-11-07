package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListAllTasks -.
func (c *Controller) ListAllTasks(ctx context.Context, req *v1.ListTasksRequest) (*v1.ListTasksResponse, error) {
	caller, ok := grpcmw.PrincipalFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	return listPage(ctx, c, query.ListAllTasks{
		Principal: caller, Status: listStatus(req), Limit: int(req.GetLimit()), Offset: int(req.GetOffset()),
	}, "ListAllTasks")
}
