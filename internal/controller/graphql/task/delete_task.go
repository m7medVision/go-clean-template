package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

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
