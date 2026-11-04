package worker

import (
	v1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// WithMail delivers the emails enqueued on the mail queue, by sending DeliverEmail on cmds.
func WithMail(cmds *cqrs.Commands) Option {
	return func(w *worker.Worker, l logger.Interface) {
		v1.NewMailRoutes(w, cmds, l)
	}
}
