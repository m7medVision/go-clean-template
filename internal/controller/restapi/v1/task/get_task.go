package task

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
)

func (r *controller) getTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-task",
		Summary:     "Get task",
		Description: "Get a task by ID",
		Method:      http.MethodGet,
		Path:        "/v1/tasks/{id}",
		Tags:        []string{_tasksTag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound),
	}, func(ctx context.Context, input *taskPathInput) (*taskOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		task, err := r.readBack(ctx, userID, input.ID)
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "getTask")
		}

		return &taskOutput{Body: task}, nil
	})
}
