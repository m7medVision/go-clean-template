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

type transitionTaskInput struct {
	ID   string `path:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Body TransitionTask
}

func (r *controller) transitionTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "transition-task",
		Summary:     "Transition task status",
		Description: "Change task status (todo -> in_progress -> done, or in_progress -> todo)",
		Method:      http.MethodPatch,
		Path:        "/v1/tasks/{id}/status",
		Tags:        []string{_tasksTag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.CommandErrors(http.StatusUnauthorized, http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound),
	}, func(ctx context.Context, input *transitionTaskInput) (*taskOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		_, err := cqrs.SendCommand(ctx, r.cmds, command.TransitionTask{
			UserID: userID, TaskID: input.ID, Status: input.Body.Status,
		})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "transitionTask")
		}

		task, err := r.readBack(ctx, userID, input.ID)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "transitionTask")
		}

		return &taskOutput{Body: task}, nil
	})
}
