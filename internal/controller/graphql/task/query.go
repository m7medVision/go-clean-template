package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/model"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/paging"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Task resolves Query.task.
func (q Query) Task(ctx context.Context, id string) (*entity.Task, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	task, err := cqrs.SendQuery[entity.Task](ctx, q.qs, query.GetTask{UserID: caller.UserID, TaskID: id})
	if err != nil {
		return nil, gqlerr.Map(q.l, err, "task")
	}

	return &task, nil
}

// Tasks resolves Query.tasks.
func (q Query) Tasks(ctx context.Context, status *entity.TaskStatus, limit, offset int) (*model.TaskList, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	if !paging.Valid(limit, offset) {
		return nil, gqlerr.InvalidInput()
	}

	return page(ctx, q, query.ListTasks{UserID: caller.UserID, Status: status, Limit: limit, Offset: offset}, "tasks")
}

// AllTasks resolves Query.allTasks. The query bus checks the admin role.
func (q Query) AllTasks(ctx context.Context, status *entity.TaskStatus, limit, offset int) (*model.TaskList, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	if !paging.Valid(limit, offset) {
		return nil, gqlerr.InvalidInput()
	}

	return page(ctx, q, query.ListAllTasks{Principal: caller, Status: status, Limit: limit, Offset: offset}, "allTasks")
}

// page sends a list query and returns its page.
func page[L cqrs.Query](ctx context.Context, q Query, list L, op string) (*model.TaskList, error) {
	p, err := cqrs.SendQuery[query.TaskPage](ctx, q.qs, list)
	if err != nil {
		return nil, gqlerr.Map(q.l, err, op)
	}

	return &model.TaskList{Tasks: p.Tasks, Total: p.Total}, nil
}
