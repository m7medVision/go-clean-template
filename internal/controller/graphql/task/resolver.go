// Package task resolves the task fields of the GraphQL schema: each field in the file named as its
// schema file is (task.go resolves schema/task.graphqls).
package task

import (
	"context"
	"log/slog"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/gqlerr"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql/model"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/task/query"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type Query struct {
	qs *cqrs.Queries
	l  *slog.Logger
}

type Mutation struct {
	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

func New(b cqrs.Buses, l *slog.Logger) (Query, Mutation) {
	return Query{qs: b.Queries, l: l}, Mutation{cmds: b.Commands, qs: b.Queries, l: l}
}

func page[L cqrs.QueryOf[entity.List[entity.Task]]](ctx context.Context, q Query, list L, op string) (*model.TaskList, error) {
	p, err := cqrs.SendQuery(ctx, q.qs, list)
	if err != nil {
		return nil, gqlerr.Map(ctx, q.l, err, op)
	}

	return &model.TaskList{Tasks: p.Items, Total: p.Total}, nil
}

// result reads back the task a mutation wrote, as its answer.
func (m Mutation) result(ctx context.Context, userID, taskID, op string) (*entity.Task, error) {
	task, err := cqrs.SendQuery(ctx, m.qs, query.GetTask{UserID: userID, TaskID: taskID})
	if err != nil {
		return nil, gqlerr.Map(ctx, m.l, err, op)
	}

	return &task, nil
}

// deref returns the description a mutation was given, or "" when it was left out.
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
