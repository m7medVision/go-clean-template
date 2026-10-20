package v1

import (
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// V1 -.
type V1 struct {
	e  usecase.Exchange
	u  usecase.User
	tk usecase.Task
	l  logger.Interface
}
