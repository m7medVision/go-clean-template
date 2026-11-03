package grpc

import (
	v1 "github.com/m7medvision/go-clean-template/internal/controller/grpc/v1"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	pbgrpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// NewRouter registers every version's services, and server reflection.
func NewRouter(app *pbgrpc.Server, b cqrs.Buses, e usecase.Exchange, l logger.Interface) {
	v1.NewRoutes(app, b, e, l)

	reflection.Register(app)
}
