package v1

import (
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// ExchangeController -.
type ExchangeController struct {
	v1.UnimplementedExchangeServiceServer

	e usecase.Exchange
	l logger.Interface
}
