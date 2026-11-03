// Package task resolves the task fields of the GraphQL schema (schema/task.graphqls).
package task

import (
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// Query resolves the task fields of Query.
type Query struct {
	task usecase.Task
	l    logger.Interface
}

// Mutation resolves the task fields of Mutation.
type Mutation struct {
	task usecase.Task
	l    logger.Interface
}

// New returns the task resolvers for Query and Mutation.
func New(uc usecase.Task, l logger.Interface) (Query, Mutation) {
	return Query{task: uc, l: l}, Mutation{task: uc, l: l}
}
