package app

import (
	"log/slog"

	grpcctl "github.com/m7medvision/go-clean-template/internal/controller/grpc"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

// newGRPCServer builds the gRPC server with every v1 service registered over b.
func newGRPCServer(b cqrs.Buses, jwtManager *jwt.Manager, l *slog.Logger) *grpc.Server {
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(grpcmw.AuthInterceptor(jwtManager)),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	grpcctl.NewRouter(srv, b, l)

	return srv
}
