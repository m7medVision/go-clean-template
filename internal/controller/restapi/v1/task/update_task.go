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

type updateTaskInput struct {
	ID   string `path:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Body UpdateTask
}

func (r *controller) updateTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "update-task",
		Summary:     "Update task",
		Description: "Update task title and description",
		Method:      http.MethodPut,
		Path:        "/v1/tasks/{id}",
		Tags:        []string{_tag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.CommandErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound),
	}, func(ctx context.Context, input *updateTaskInput) (*taskOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		_, err := cqrs.SendCommand(ctx, r.cmds, command.UpdateTask{
			UserID: userID, TaskID: input.ID, Title: input.Body.Title, Description: input.Body.Description,
		})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "updateTask")
		}

		task, err := r.readBack(ctx, userID, input.ID)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "updateTask")
		}

		return &taskOutput{Body: task}, nil
	})
}
