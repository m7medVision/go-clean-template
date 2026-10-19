package v1

import (
	v1 "github.com/evrone/go-clean-template/docs/proto/v1"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/go-playground/validator/v10"
	pbgrpc "google.golang.org/grpc"
)

// NewExchangeRoutes -.
func NewExchangeRoutes(app *pbgrpc.Server, e usecase.Exchange, l logger.Interface) {
	v1.RegisterExchangeServiceServer(app, &ExchangeController{e: e, l: l})
}

// NewAuthRoutes -.
func NewAuthRoutes(app *pbgrpc.Server, u usecase.User, l logger.Interface) {
	r := &AuthController{u: u, l: l, v: validator.New(validator.WithRequiredStructEnabled())}

	v1.RegisterAuthServiceServer(app, r)
}

// NewTaskRoutes -.
func NewTaskRoutes(app *pbgrpc.Server, tk usecase.Task, l logger.Interface) {
	r := &TaskController{tk: tk, l: l, v: validator.New(validator.WithRequiredStructEnabled())}

	v1.RegisterTaskServiceServer(app, r)
}
