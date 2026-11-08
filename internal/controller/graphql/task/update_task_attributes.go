package task

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/model"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (m Mutation) UpdateTaskAttributes(
	ctx context.Context, id string, input model.TaskAttributesInput,
) (*mongotask.TaskWithAttributes, error) {
	caller, err := middleware.Caller(ctx)
	if err != nil {
		return nil, err
	}

	_, err = cqrs.SendCommand(ctx, m.cmds, command.SetTaskAttributes{
		UserID: caller.UserID, TaskID: id, Patch: mongotask.AttributesPatch{
			Priority: field(input.Priority), Estimate: field(input.Estimate), Labels: sliceField(input.Labels),
		},
	})
	if err != nil {
		return nil, gqlerr.Map(ctx, m.l, err, "updateTaskAttributes")
	}

	return m.result(ctx, caller.UserID, id, "updateTaskAttributes")
}

func field[T any](o graphql.Omittable[*T]) mongotask.Field[T] {
	v, set := o.ValueOK()

	return mongotask.Field[T]{Set: set, Value: v}
}

func sliceField(o graphql.Omittable[[]string]) mongotask.Field[[]string] {
	v, set := o.ValueOK()
	if v == nil {
		return mongotask.Field[[]string]{Set: set}
	}

	return mongotask.Field[[]string]{Set: set, Value: &v}
}
