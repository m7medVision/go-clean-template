package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/model"
	"github.com/m7medvision/go-clean-template/internal/entity"
)

// CreateTask resolves Mutation.createTask.
func (m Mutation) CreateTask(ctx context.Context, input model.CreateTaskInput) (*entity.Task, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	if !validInput(input.Title, input.Description) {
		return nil, gqlerr.InvalidInput()
	}

	task, err := m.task.Create(ctx, caller.UserID, input.Title, deref(input.Description))
	if err != nil {
		return nil, gqlerr.Map(m.l, err, "createTask")
	}

	return &task, nil
}

// UpdateTask resolves Mutation.updateTask.
func (m Mutation) UpdateTask(ctx context.Context, id string, input model.UpdateTaskInput) (*entity.Task, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	if !validInput(input.Title, input.Description) {
		return nil, gqlerr.InvalidInput()
	}

	task, err := m.task.Update(ctx, caller.UserID, id, input.Title, deref(input.Description))
	if err != nil {
		return nil, gqlerr.Map(m.l, err, "updateTask")
	}

	return &task, nil
}

// TransitionTask resolves Mutation.transitionTask. The entity decides which moves are allowed.
func (m Mutation) TransitionTask(ctx context.Context, id string, status entity.TaskStatus) (*entity.Task, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	task, err := m.task.Transition(ctx, caller.UserID, id, status)
	if err != nil {
		return nil, gqlerr.Map(m.l, err, "transitionTask")
	}

	return &task, nil
}

// DeleteTask resolves Mutation.deleteTask.
func (m Mutation) DeleteTask(ctx context.Context, id string) (bool, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return false, err
	}

	if err := m.task.Delete(ctx, caller.UserID, id); err != nil {
		return false, gqlerr.Map(m.l, err, "deleteTask")
	}

	return true, nil
}
