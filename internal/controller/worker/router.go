// Package worker registers the queue handlers the worker binary consumes.
package worker

import (
	v1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// Option registers an optional feature's queue handlers.
type Option func(w *worker.Worker, l logger.Interface)

// NewRouter registers the example handler, then each option's handlers.
func NewRouter(w *worker.Worker, l logger.Interface, opts ...Option) {
	v1.NewExampleRoutes(w, l)

	for _, opt := range opts {
		opt(w, l)
	}
}
