package task

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type updateTaskAttributesInput struct {
	ID   string `path:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Body UpdateTaskAttributes
}

func (r *controller) updateTaskAttributes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "update-task-attributes",
		Summary:     "Update task attributes",
		Description: "Set or clear a task's attributes: a field left out stays as it is, null clears it",
		Method:      http.MethodPatch,
		Path:        "/v1/tasks/{id}/attributes",
		Tags:        []string{_tag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.CommandErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound),
	}, func(ctx context.Context, input *updateTaskAttributesInput) (*taskOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		_, err := cqrs.SendCommand(ctx, r.cmds, command.SetTaskAttributes{
			UserID: userID, TaskID: input.ID, Patch: input.Body.patch(),
		})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "updateTaskAttributes")
		}

		task, err := r.readBack(ctx, userID, input.ID)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "updateTaskAttributes")
		}

		return &taskOutput{Body: task}, nil
	})
}
