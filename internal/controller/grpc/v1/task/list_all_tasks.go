package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
)

// ListAllTasks -.
func (c *Controller) ListAllTasks(ctx context.Context, req *v1.ListTasksRequest) (*v1.ListTasksResponse, error) {
	return listPage(ctx, c, query.ListAllTasks{
		Principal: authn.MustCaller(ctx).Principal, Status: listStatus(req), Limit: int(req.GetLimit()), Offset: int(req.GetOffset()),
	}, "ListAllTasks")
}
