package v1

import (
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/evrone/go-clean-template/pkg/worker"
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

// NewMailController -.
func NewMailController(m usecase.Mail, l logger.Interface) *MailController {
	return &MailController{mail: m, l: l}
}

// NewMailRoutes -.
func NewMailRoutes(w *worker.Worker, m usecase.Mail, l logger.Interface) {
	w.Handle(entity.EmailQueue, NewMailController(m, l).Handle)
}
