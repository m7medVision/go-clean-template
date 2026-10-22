package v1

import (
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// ExampleQueue is the queue the example handler consumes.
const ExampleQueue = "example"

// NewExampleController -.
func NewExampleController(l logger.Interface) *ExampleController {
	return &ExampleController{l: l}
}

// NewExampleRoutes -.
func NewExampleRoutes(w *worker.Worker, l logger.Interface) {
	w.Handle(ExampleQueue, NewExampleController(l).Handle)
}
