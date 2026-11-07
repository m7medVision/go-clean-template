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

// newGRPCServer only traces and recovers streaming calls: server reflection, the one streaming
// service, needs no token.
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

func publicGRPCMethods() []string {
	var public []string

	for _, m := range modules() {
		public = append(public, m.grpcPublic...)
	}

	return public
}
