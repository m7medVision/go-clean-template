package v1

import (
	"log/slog"

	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// ExampleQueue is the queue the example handler consumes.
const ExampleQueue = "example"

// NewExampleController -.
func NewExampleController(l *slog.Logger) *ExampleController {
	return &ExampleController{l: l}
}

// NewExampleRoutes -.
func NewExampleRoutes(w *worker.Worker, l *slog.Logger) {
	w.Handle(ExampleQueue, NewExampleController(l).Handle)
}
