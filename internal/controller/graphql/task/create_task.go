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
