package worker

import (
	v1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// WithMail delivers the emails use cases enqueue on the mail queue.
func WithMail(m usecase.Mail) Option {
	return func(w *worker.Worker, l logger.Interface) {
		v1.NewMailRoutes(w, m, l)
	}
}
