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

const _tag = "tasks"

type controller struct {
	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

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

func (r *controller) readBack(ctx context.Context, userID, taskID string) (entity.Task, error) {
	return cqrs.SendQuery(ctx, r.qs, query.GetTask{UserID: userID, TaskID: taskID})
}

func listPage[Q cqrs.QueryOf[entity.List[entity.Task]]](ctx context.Context, r *controller, q Q, op string) (*listTasksOutput, error) {
	page, err := cqrs.SendQuery(ctx, r.qs, q)
	if err != nil {
		return nil, apierr.Problem(ctx, r.l, err, op)
	}

	return &listTasksOutput{Body: TaskList{Tasks: page.Items, Total: page.Total}}, nil
}
