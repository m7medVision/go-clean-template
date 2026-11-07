package task

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
)

func (r *controller) listAllTasks(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-all-tasks",
		Summary:     "List every user's tasks",
		Description: "List tasks across all users with optional filtering. Requires the admin role.",
		Method:      http.MethodGet,
		Path:        "/v1/admin/tasks",
		Tags:        []string{_tag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized, http.StatusForbidden),
	}, func(ctx context.Context, input *listTasksInput) (*listTasksOutput, error) {
		return listPage(ctx, r, query.ListAllTasks{
			Principal: authn.MustCaller(ctx).Principal, Status: entity.TaskStatusFilter(input.Status),
			Limit: input.Limit, Offset: input.Offset,
		}, "listAllTasks")
	})
}
