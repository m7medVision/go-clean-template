package v1

import (
	v1 "github.com/evrone/go-clean-template/docs/proto/v1"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
)

// ExchangeController -.
type ExchangeController struct {
	v1.UnimplementedExchangeServiceServer

	e usecase.Exchange
	l logger.Interface
}
