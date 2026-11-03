// Package task serves the v1 task operations over REST.
package task

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

const _tasksTag = "tasks"

type controller struct {
	tk usecase.Task
	l  logger.Interface
}

// NewRoutes registers the task operations.
func NewRoutes(api huma.API, tk usecase.Task, l logger.Interface) {
	r := &controller{tk: tk, l: l}

	r.createTask(api)
	r.listTasks(api)
	r.listAllTasks(api)
	r.getTask(api)
	r.updateTask(api)
	r.transitionTask(api)
	r.deleteTask(api)
}
