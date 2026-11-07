package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/model"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
)

// AllTasks leaves the admin role check to the query bus.
func (q Query) AllTasks(ctx context.Context, status *entity.TaskStatus, limit, offset int) (*model.TaskList, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	return page(ctx, q, query.ListAllTasks{Principal: caller.Principal, Status: status, Limit: limit, Offset: offset}, "allTasks")
}
