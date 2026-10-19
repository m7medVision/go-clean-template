package v1

import (
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
)

// V1 -.
type V1 struct {
	u  usecase.User
	tk usecase.Task
	l  logger.Interface
}
