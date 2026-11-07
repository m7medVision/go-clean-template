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

type deleteTaskOutput struct {
	Status int
}

func (r *controller) deleteTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "delete-task",
		Summary:       "Delete task",
		Description:   "Delete a task by ID",
		Method:        http.MethodDelete,
		Path:          "/v1/tasks/{id}",
		Tags:          []string{_tasksTag},
		Security:      []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		DefaultStatus: http.StatusNoContent,
		Errors:        apierr.CommandErrors(http.StatusUnauthorized, http.StatusNotFound),
	}, func(ctx context.Context, input *taskPathInput) (*deleteTaskOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		_, err := cqrs.SendCommand(ctx, r.cmds, command.DeleteTask{UserID: userID, TaskID: input.ID})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "deleteTask")
		}

		return &deleteTaskOutput{Status: http.StatusNoContent}, nil
	})
}
