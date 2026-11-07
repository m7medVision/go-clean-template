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
