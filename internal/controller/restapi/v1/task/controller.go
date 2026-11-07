// Package task serves the v1 task operations over REST.
package task

import (
	"context"
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
		return nil, apierr.Problem(ctx, r.l, err, op)
	}

	return &listTasksOutput{Body: TaskList{Tasks: page.Tasks, Total: page.Total}}, nil
}
