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

type createTaskInput struct {
	Body CreateTask
}

func (r *controller) createTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-task",
		Summary:       "Create task",
		Description:   "Create a new task for the current user",
		Method:        http.MethodPost,
		Path:          "/v1/tasks",
		Tags:          []string{_tag},
		Security:      []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		DefaultStatus: http.StatusCreated,
		Errors:        apierr.CommandErrors(http.StatusUnauthorized),
	}, func(ctx context.Context, input *createTaskInput) (*taskOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		written, err := cqrs.SendCommand(ctx, r.cmds, command.CreateTask{
			UserID: userID, Title: input.Body.Title, Description: input.Body.Description,
		})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "createTask")
		}

		task, err := r.readBack(ctx, userID, written.ID)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "createTask")
		}

		return &taskOutput{Body: task}, nil
	})
}
