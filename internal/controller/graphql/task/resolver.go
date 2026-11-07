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

// Query resolves the task fields of Query.
type Query struct {
	qs *cqrs.Queries
	l  *slog.Logger
}

// Mutation resolves the task fields of Mutation.
type Mutation struct {
	cmds *cqrs.Commands
	qs   *cqrs.Queries
	l    *slog.Logger
}

// New returns the task resolvers for Query and Mutation.
func New(b cqrs.Buses, l *slog.Logger) (Query, Mutation) {
	return Query{qs: b.Queries, l: l}, Mutation{cmds: b.Commands, qs: b.Queries, l: l}
}

// readBack reads the task a command just wrote, for the mutation's result.
func (m Mutation) readBack(ctx context.Context, userID, taskID string) (entity.Task, error) {
	return cqrs.SendQuery(ctx, m.qs, query.GetTask{UserID: userID, TaskID: taskID})
}

// page sends a list query and returns its page.
func page[L cqrs.QueryOf[entity.List[entity.Task]]](ctx context.Context, q Query, list L, op string) (*model.TaskList, error) {
	p, err := cqrs.SendQuery(ctx, q.qs, list)
	if err != nil {
		return nil, gqlerr.Map(ctx, q.l, err, op)
	}

	return &model.TaskList{Tasks: p.Items, Total: p.Total}, nil
}

// result reads the task a mutation wrote back, as its result.
func (m Mutation) result(ctx context.Context, userID, taskID, op string) (*entity.Task, error) {
	task, err := m.readBack(ctx, userID, taskID)
	if err != nil {
		return nil, gqlerr.Map(ctx, m.l, err, op)
	}

	return &task, nil
}
