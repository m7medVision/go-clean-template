package app

import (
	"log/slog"

	"github.com/m7medvision/go-clean-template/config"
	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// newGRPCServer builds the gRPC server with every module's services registered over b, and server
// reflection. Every unary call is traced, logged, counted when metrics are on, recovered from a
// panic, and needs a token jwtManager signed unless a module lists its method as public. Server
// reflection, the one streaming service, is only traced and recovered: it needs no token.
func newGRPCServer(cfg *config.Config, b cqrs.Buses, jwtManager *jwt.Manager, l *slog.Logger) *grpc.Server {
	unary := []grpc.UnaryServerInterceptor{grpcmw.Logger(l)}

	if cfg.Metrics.Enabled {
		unary = append(unary, grpcmw.NewMetrics(prometheus.DefaultRegisterer).Interceptor)
	}

	unary = append(unary, grpcmw.Recovery(l), grpcmw.Auth(jwtManager, publicGRPCMethods()...))

	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(grpcmw.StreamRecovery(l)),
	)

	registerGRPC(srv, b, l)
	reflection.Register(srv)

	return srv
}

// publicGRPCMethods lists every module's gRPC methods a caller reaches without a token.
func publicGRPCMethods() []string {
	var public []string

	for _, m := range modules() {
		public = append(public, m.grpcPublic...)
	}

	return public
}
