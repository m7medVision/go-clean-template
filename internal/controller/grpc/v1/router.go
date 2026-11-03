package v1

import (
	v1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/exchange"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/task"
	"github.com/m7medvision/go-clean-template/internal/controller/grpc/v1/user"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	pbgrpc "google.golang.org/grpc"
)

// NewRoutes registers every v1 service, one line per domain.
func NewRoutes(app *pbgrpc.Server, b cqrs.Buses, e usecase.Exchange, l logger.Interface) {
	v1.RegisterAuthServiceServer(app, user.New(b, l))
	v1.RegisterTaskServiceServer(app, task.New(b, l))
	v1.RegisterExchangeServiceServer(app, exchange.New(e, l))
}
