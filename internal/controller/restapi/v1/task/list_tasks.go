package task

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
)

func (r *controller) listTasks(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-tasks",
		Summary:     "List tasks",
		Description: "List tasks for the current user with optional filtering",
		Method:      http.MethodGet,
		Path:        "/v1/tasks",
		Tags:        []string{_tag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized),
	}, func(ctx context.Context, input *listTasksInput) (*listTasksOutput, error) {
		userID := authn.MustCaller(ctx).UserID

		return listPage(ctx, r, query.ListTasks{
			UserID: userID, Status: input.status(), Limit: input.Limit, Offset: input.Offset,
		}, "listTasks")
	})
}
