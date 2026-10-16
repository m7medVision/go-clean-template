package v1

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

type createTaskInput struct {
	Body request.CreateTask
}

type createTaskOutput struct {
	Body entity.Task
}

type listTasksInput struct {
	Status entity.TaskStatus `query:"status" required:"false" enum:"todo,in_progress,done" example:"todo"`
	Limit  int               `query:"limit" default:"10" minimum:"1"`
	Offset int               `query:"offset" default:"0" minimum:"0"`
}

type listTasksOutput struct {
	Body response.TaskList
}

type taskPathInput struct {
	ID string `path:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type updateTaskInput struct {
	ID   string `path:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Body request.UpdateTask
}

type updateTaskOutput struct {
	Body entity.Task
}

type transitionTaskInput struct {
	ID   string `path:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Body request.TransitionTask
}

type transitionTaskOutput struct {
	Body entity.Task
}

type deleteTaskOutput struct {
	Status int
}

func (r *V1) createTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-task",
		Summary:       "Create task",
		Description:   "Create a new task for the current user",
		Method:        http.MethodPost,
		Path:          "/v1/tasks",
		Tags:          []string{_tasksTag},
		Security:      []map[string][]string{{_bearerAuthScheme: {}}},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, func(ctx context.Context, input *createTaskInput) (*createTaskOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		task, err := r.tk.Create(ctx, userID, input.Body.Title, input.Body.Description)
		if err != nil {
			r.l.Error(err, "restapi - v1 - createTask")

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &createTaskOutput{Body: task}, nil
	})
}

func (r *V1) listTasks(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-tasks",
		Summary:     "List tasks",
		Description: "List tasks for the current user with optional filtering",
		Method:      http.MethodGet,
		Path:        "/v1/tasks",
		Tags:        []string{_tasksTag},
		Security:    []map[string][]string{{_bearerAuthScheme: {}}},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, func(ctx context.Context, input *listTasksInput) (*listTasksOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		var status *entity.TaskStatus

		if input.Status != "" {
			local := input.Status
			status = &local
		}

		tasks, total, err := r.tk.List(ctx, userID, status, input.Limit, input.Offset)
		if err != nil {
			r.l.Error(err, "restapi - v1 - listTasks")

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &listTasksOutput{Body: response.TaskList{Tasks: tasks, Total: total}}, nil
	})
}

func (r *V1) getTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-task",
		Summary:     "Get task",
		Description: "Get a task by ID",
		Method:      http.MethodGet,
		Path:        "/v1/tasks/{id}",
		Tags:        []string{_tasksTag},
		Security:    []map[string][]string{{_bearerAuthScheme: {}}},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}, func(ctx context.Context, input *taskPathInput) (*updateTaskOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		task, err := r.tk.Get(ctx, userID, input.ID)
		if err != nil {
			r.l.Error(err, "restapi - v1 - getTask")

			if errors.Is(err, entity.ErrTaskForbidden) {
				return nil, huma.Error403Forbidden("forbidden")
			}

			if errors.Is(err, entity.ErrTaskNotFound) {
				return nil, huma.Error404NotFound("task not found")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &updateTaskOutput{Body: task}, nil
	})
}

func (r *V1) updateTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "update-task",
		Summary:     "Update task",
		Description: "Update task title and description",
		Method:      http.MethodPut,
		Path:        "/v1/tasks/{id}",
		Tags:        []string{_tasksTag},
		Security:    []map[string][]string{{_bearerAuthScheme: {}}},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}, func(ctx context.Context, input *updateTaskInput) (*updateTaskOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		task, err := r.tk.Update(ctx, userID, input.ID, input.Body.Title, input.Body.Description)
		if err != nil {
			r.l.Error(err, "restapi - v1 - updateTask")

			if errors.Is(err, entity.ErrTaskForbidden) {
				return nil, huma.Error403Forbidden("forbidden")
			}

			if errors.Is(err, entity.ErrTaskNotFound) {
				return nil, huma.Error404NotFound("task not found")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &updateTaskOutput{Body: task}, nil
	})
}

func (r *V1) transitionTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "transition-task",
		Summary:     "Transition task status",
		Description: "Change task status (todo -> in_progress -> done, or in_progress -> todo)",
		Method:      http.MethodPatch,
		Path:        "/v1/tasks/{id}/status",
		Tags:        []string{_tasksTag},
		Security:    []map[string][]string{{_bearerAuthScheme: {}}},
		Errors:      []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}, func(ctx context.Context, input *transitionTaskInput) (*transitionTaskOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		task, err := r.tk.Transition(ctx, userID, input.ID, input.Body.Status)
		if err != nil {
			r.l.Error(err, "restapi - v1 - transitionTask")

			if errors.Is(err, entity.ErrInvalidTransition) {
				return nil, huma.Error400BadRequest("invalid status transition")
			}

			if errors.Is(err, entity.ErrTaskForbidden) {
				return nil, huma.Error403Forbidden("forbidden")
			}

			if errors.Is(err, entity.ErrTaskNotFound) {
				return nil, huma.Error404NotFound("task not found")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &transitionTaskOutput{Body: task}, nil
	})
}

func (r *V1) deleteTask(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "delete-task",
		Summary:       "Delete task",
		Description:   "Delete a task by ID",
		Method:        http.MethodDelete,
		Path:          "/v1/tasks/{id}",
		Tags:          []string{_tasksTag},
		Security:      []map[string][]string{{_bearerAuthScheme: {}}},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusNotFound, http.StatusInternalServerError},
	}, func(ctx context.Context, input *taskPathInput) (*deleteTaskOutput, error) {
		userID, ok := middleware.UserID(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		err := r.tk.Delete(ctx, userID, input.ID)
		if err != nil {
			r.l.Error(err, "restapi - v1 - deleteTask")

			if errors.Is(err, entity.ErrTaskNotFound) {
				return nil, huma.Error404NotFound("task not found")
			}

			return nil, huma.Error500InternalServerError("internal server error")
		}

		return &deleteTaskOutput{Status: http.StatusNoContent}, nil
	})
}
