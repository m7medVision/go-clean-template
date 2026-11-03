package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/model"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/paging"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

// Task resolves Query.task.
func (q Query) Task(ctx context.Context, id string) (*entity.Task, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	task, err := q.task.Get(ctx, caller.UserID, id)
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

	tasks, total, err := q.task.List(ctx, caller.UserID, status, limit, offset)
	if err != nil {
		return nil, gqlerr.Map(q.l, err, "tasks")
	}

	return &model.TaskList{Tasks: tasks, Total: total}, nil
}

// AllTasks resolves Query.allTasks. The use case checks the admin role.
func (q Query) AllTasks(ctx context.Context, status *entity.TaskStatus, limit, offset int) (*model.TaskList, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	if !paging.Valid(limit, offset) {
		return nil, gqlerr.InvalidInput()
	}

	tasks, total, err := q.task.ListAll(ctx, caller, status, limit, offset)
	if err != nil {
		return nil, gqlerr.Map(q.l, err, "allTasks")
	}

	return &model.TaskList{Tasks: tasks, Total: total}, nil
}
