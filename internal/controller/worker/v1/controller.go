package v1

import (
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
)

// ExampleController -.
type ExampleController struct {
	l logger.Interface
}

// MailController -.
type MailController struct {
	mail usecase.Mail
	l    logger.Interface
}
