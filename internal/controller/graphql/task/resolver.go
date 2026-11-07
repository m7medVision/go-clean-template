// Package task resolves the task fields of the GraphQL schema (schema/task.graphqls).
package task

import (
	"context"
	"log/slog"

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
	return cqrs.SendQuery[entity.Task](ctx, m.qs, query.GetTask{UserID: userID, TaskID: taskID})
}
