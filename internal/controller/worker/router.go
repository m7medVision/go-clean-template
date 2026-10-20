// Package worker registers the queue handlers the worker binary consumes.
package worker

import (
	v1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// NewRouter -.
func NewRouter(w *worker.Worker, m usecase.Mail, l logger.Interface) {
	{
		v1.NewExampleRoutes(w, l)
		v1.NewMailRoutes(w, m, l)
	}
}
