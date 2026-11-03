package v1

import (
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/exchange"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/task"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/user"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	pbgrpc "google.golang.org/grpc"
)

// NewExchangeRoutes -.
func NewExchangeRoutes(app *pbgrpc.Server, e usecase.Exchange, l logger.Interface) {
	v1.RegisterExchangeServiceServer(app, exchange.New(e, l))
}

// NewAuthRoutes -.
func NewAuthRoutes(app *pbgrpc.Server, u usecase.User, l logger.Interface) {
	v1.RegisterAuthServiceServer(app, user.New(u, l))
}

// NewTaskRoutes -.
func NewTaskRoutes(app *pbgrpc.Server, tk usecase.Task, l logger.Interface) {
	v1.RegisterTaskServiceServer(app, task.New(tk, l))
}
