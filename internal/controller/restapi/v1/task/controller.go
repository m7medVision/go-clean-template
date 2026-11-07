// Package task serves the v1 task operations over REST.
package task

import (
	"context"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apierr"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

const _tasksTag = "tasks"

type controller struct {
	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

// NewRoutes registers the task operations.
func NewRoutes(api huma.API, b cqrs.Buses, l *slog.Logger) {
	r := &controller{cmds: b.Commands, qs: b.Queries, l: l}

	r.createTask(api)
	r.listTasks(api)
	r.listAllTasks(api)
	r.getTask(api)
	r.updateTask(api)
	r.transitionTask(api)
	r.deleteTask(api)
}

// readBack reads the task a command just wrote, for the response body.
func (r *controller) readBack(ctx context.Context, userID, taskID string) (entity.Task, error) {
	return cqrs.SendQuery[entity.Task](ctx, r.qs, query.GetTask{UserID: userID, TaskID: taskID})
}

// listPage sends a list query and answers with its page.
func listPage[Q cqrs.Query](ctx context.Context, r *controller, q Q, op string) (*listTasksOutput, error) {
	page, err := cqrs.SendQuery[query.TaskPage](ctx, r.qs, q)
	if err != nil {
		return nil, r.problem(err, op)
	}

	return &listTasksOutput{Body: TaskList{Tasks: page.Tasks, Total: page.Total}}, nil
}

// problem maps a failed command or query to its REST problem: first what any command can fail
// with, then the task errors, and anything else to a logged 500.
func (r *controller) problem(err error, op string) error {
	if shared := apierr.Shared(err); shared != nil {
		r.l.Warn("restapi - v1 - "+op, "error", err)

		return shared
	}

	switch {
	case errors.Is(err, entity.ErrInvalidTransition):
		return huma.Error400BadRequest("invalid status transition")
	case errors.Is(err, entity.ErrAdminRequired):
		return huma.Error403Forbidden("admin role required")
	case errors.Is(err, entity.ErrTaskForbidden):
		return huma.Error403Forbidden("forbidden")
	case errors.Is(err, entity.ErrTaskNotFound):
		return huma.Error404NotFound("task not found")
	default:
		r.l.Error("restapi - v1 - "+op, "error", err)

		return huma.Error500InternalServerError("internal server error")
	}
}
