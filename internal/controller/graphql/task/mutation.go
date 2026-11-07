package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/model"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// CreateTask resolves Mutation.createTask.
func (m Mutation) CreateTask(ctx context.Context, input model.CreateTaskInput) (*entity.Task, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	written, err := cqrs.SendCommand(ctx, m.cmds, command.CreateTask{
		UserID: caller.UserID, Title: input.Title, Description: deref(input.Description),
	})
	if err != nil {
		return nil, gqlerr.Map(ctx, m.l, err, "createTask")
	}

	return m.result(ctx, caller.UserID, written.ID, "createTask")
}

// UpdateTask resolves Mutation.updateTask.
func (m Mutation) UpdateTask(ctx context.Context, id string, input model.UpdateTaskInput) (*entity.Task, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	_, err = cqrs.SendCommand(ctx, m.cmds, command.UpdateTask{
		UserID: caller.UserID, TaskID: id, Title: input.Title, Description: deref(input.Description),
	})
	if err != nil {
		return nil, gqlerr.Map(ctx, m.l, err, "updateTask")
	}

	return m.result(ctx, caller.UserID, id, "updateTask")
}

// TransitionTask resolves Mutation.transitionTask. The aggregate decides which moves are allowed.
func (m Mutation) TransitionTask(ctx context.Context, id string, status entity.TaskStatus) (*entity.Task, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	_, err = cqrs.SendCommand(ctx, m.cmds, command.TransitionTask{
		UserID: caller.UserID, TaskID: id, Status: status,
	})
	if err != nil {
		return nil, gqlerr.Map(ctx, m.l, err, "transitionTask")
	}

	return m.result(ctx, caller.UserID, id, "transitionTask")
}

// DeleteTask resolves Mutation.deleteTask.
func (m Mutation) DeleteTask(ctx context.Context, id string) (bool, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return false, err
	}

	_, err = cqrs.SendCommand(ctx, m.cmds, command.DeleteTask{UserID: caller.UserID, TaskID: id})
	if err != nil {
		return false, gqlerr.Map(ctx, m.l, err, "deleteTask")
	}

	return true, nil
}

// result reads the task a mutation wrote back, as its result.
func (m Mutation) result(ctx context.Context, userID, taskID, op string) (*entity.Task, error) {
	task, err := m.readBack(ctx, userID, taskID)
	if err != nil {
		return nil, gqlerr.Map(ctx, m.l, err, op)
	}

	return &task, nil
}
