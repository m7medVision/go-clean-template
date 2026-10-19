// Package worker registers the queue handlers the worker binary consumes.
package worker

import (
	v1 "github.com/evrone/go-clean-template/internal/controller/worker/v1"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/evrone/go-clean-template/pkg/worker"
)

// NewRouter -.
func NewRouter(w *worker.Worker, l logger.Interface) {
	{
		v1.NewExampleRoutes(w, l)
	}
}
