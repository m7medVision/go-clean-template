package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
)

// ListTasks -.
func (c *Controller) ListTasks(ctx context.Context, req *v1.ListTasksRequest) (*v1.ListTasksResponse, error) {
	userID := authn.MustCaller(ctx).UserID

	return listPage(ctx, c, query.ListTasks{
		UserID: userID, Status: listStatus(req), Limit: int(req.GetLimit()), Offset: int(req.GetOffset()),
	}, "ListTasks")
}
