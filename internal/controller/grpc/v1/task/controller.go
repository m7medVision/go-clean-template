// Package task serves TaskService (docs/proto/v1/task.proto) over gRPC.
package task

import (
	"github.com/go-playground/validator/v10"
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// Controller -.
type Controller struct {
	v1.UnimplementedTaskServiceServer

	tk usecase.Task
	l  logger.Interface
	v  *validator.Validate
}

// New -.
func New(tk usecase.Task, l logger.Interface) v1.TaskServiceServer {
	return &Controller{tk: tk, l: l, v: validator.New(validator.WithRequiredStructEnabled())}
}
