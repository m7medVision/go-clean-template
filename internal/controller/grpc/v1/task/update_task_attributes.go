package task

import (
	"context"

	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/rpcerr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

func (c *Controller) UpdateTaskAttributes(ctx context.Context, req *v1.UpdateTaskAttributesRequest) (*v1.TaskResponse, error) {
	userID := authn.MustCaller(ctx).UserID

	patch, err := attributesPatch(req)
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "UpdateTaskAttributes")
	}

	_, err = cqrs.SendCommand(ctx, c.cmds, command.SetTaskAttributes{UserID: userID, TaskID: req.GetId(), Patch: patch})
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "UpdateTaskAttributes")
	}

	resp, err := c.readBack(ctx, userID, req.GetId())
	if err != nil {
		return nil, rpcerr.Status(ctx, c.l, err, "UpdateTaskAttributes")
	}

	return resp, nil
}

func attributesPatch(req *v1.UpdateTaskAttributesRequest) (mongotask.AttributesPatch, error) {
	var patch mongotask.AttributesPatch

	a := req.GetAttributes()

	for _, path := range req.GetUpdateMask().GetPaths() {
		switch path {
		case "priority":
			patch.Priority = mongotask.Field[mongotask.Priority]{Set: true}
			if a.Priority != nil {
				patch.Priority.Value = new(mongotask.Priority(a.GetPriority()))
			}
		case "estimate":
			patch.Estimate = mongotask.Field[float64]{Set: true, Value: a.Estimate}
		case "labels":
			patch.Labels = mongotask.Field[[]string]{Set: true}
			if len(a.GetLabels()) > 0 {
				patch.Labels.Value = new(a.GetLabels())
			}
		default:
			return mongotask.AttributesPatch{}, entity.ErrUnknownTaskAttribute
		}
	}

	return patch, nil
}
