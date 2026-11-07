package app

import (
	"log/slog"

	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// newGRPCServer builds the gRPC server with every module's services registered over b, and server
// reflection.
func newGRPCServer(b cqrs.Buses, jwtManager *jwt.Manager, l *slog.Logger) *grpc.Server {
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(grpcmw.AuthInterceptor(jwtManager)),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	registerGRPC(srv, b, l)
	reflection.Register(srv)

	return srv
}
