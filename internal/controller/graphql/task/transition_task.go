package task

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// TransitionTask leaves which moves are allowed to the aggregate.
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
