package task

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type createTaskInput struct {
	Body CreateTask
}

// taskOutput is the task an operation wrote or read.
type taskOutput struct {
	Body entity.Task
}

type listTasksInput struct {
	Status entity.TaskStatus `query:"status" required:"false" enum:"todo,in_progress,done" example:"todo"`
	Limit  int               `query:"limit" default:"10" minimum:"1" maximum:"100"`
	Offset int               `query:"offset" default:"0" minimum:"0"`
}

type listTasksOutput struct {
	Body TaskList
}

// status is the filter the query asks for, or nil for every status.
func (in *listTasksInput) status() *entity.TaskStatus {
	if in.Status == "" {
		return nil
	}

	status := in.Status

	return &status
}

type taskPathInput struct {
	ID string `path:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type updateTaskInput struct {
	ID   string `path:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Body UpdateTask
}

type transitionTaskInput struct {
	ID   string `path:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Body TransitionTask
}

type deleteTaskOutput struct {
	Status int
}

func (r *controller) createTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-task",
		Summary:       "Create task",
		Description:   "Create a new task for the current user",
		Method:        http.MethodPost,
		Path:          "/v1/tasks",
		Tags:          []string{_tasksTag},
		Security:      []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		DefaultStatus: http.StatusCreated,
		Errors:        apierr.CommandErrors(http.StatusUnauthorized),
	}, func(ctx context.Context, input *createTaskInput) (*taskOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

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

func (r *controller) listTasks(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-tasks",
		Summary:     "List tasks",
		Description: "List tasks for the current user with optional filtering",
		Method:      http.MethodGet,
		Path:        "/v1/tasks",
		Tags:        []string{_tasksTag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized),
	}, func(ctx context.Context, input *listTasksInput) (*listTasksOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		return listPage(ctx, r, query.ListTasks{
			UserID: userID, Status: input.status(), Limit: input.Limit, Offset: input.Offset,
		}, "listTasks")
	})
}

func (r *controller) listAllTasks(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-all-tasks",
		Summary:     "List every user's tasks",
		Description: "List tasks across all users with optional filtering. Requires the admin role.",
		Method:      http.MethodGet,
		Path:        "/v1/admin/tasks",
		Tags:        []string{_tasksTag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.QueryErrors(http.StatusUnauthorized, http.StatusForbidden),
	}, func(ctx context.Context, input *listTasksInput) (*listTasksOutput, error) {
		caller, ok := middleware.Principal(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		return listPage(ctx, r, query.ListAllTasks{
			Principal: caller, Status: input.status(), Limit: input.Limit, Offset: input.Offset,
		}, "listAllTasks")
	})
}

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

func (r *controller) updateTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "update-task",
		Summary:     "Update task",
		Description: "Update task title and description",
		Method:      http.MethodPut,
		Path:        "/v1/tasks/{id}",
		Tags:        []string{_tasksTag},
		Security:    []map[string][]string{{apidoc.BearerAuthScheme: {}}},
		Errors:      apierr.CommandErrors(http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound),
	}, func(ctx context.Context, input *updateTaskInput) (*taskOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

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
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

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
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		_, err := cqrs.SendCommand(ctx, r.cmds, command.DeleteTask{UserID: userID, TaskID: input.ID})
		if err != nil {
			return nil, apierr.Problem(ctx, r.l, err, "deleteTask")
		}

		return &deleteTaskOutput{Status: http.StatusNoContent}, nil
	})
}
