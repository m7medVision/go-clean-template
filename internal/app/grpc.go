package app

import (
	"log/slog"

	grpcmw "github.com/m7medvision/go-clean-template/internal/controller/grpc/middleware"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// newGRPCServer counts and times every unary call on metrics. It only traces and recovers
// streaming calls: server reflection, the one streaming service, needs no token. Reflection is the
// gRPC API's reference, so it is served only with docs, as REST's and GraphQL's are.
func newGRPCServer(b cqrs.Buses, jwtManager *jwt.Manager, metrics prometheus.Registerer, docs bool, l *slog.Logger) *grpc.Server {
	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			grpcmw.Logger(l),
			grpcmw.NewMetrics(metrics).Interceptor,
			grpcmw.Recovery(l),
			grpcmw.Auth(jwtManager, publicGRPCMethods()...),
		),
		grpc.ChainStreamInterceptor(grpcmw.StreamRecovery(l)),
	)

	registerGRPC(srv, b, l)

	if docs {
		reflection.Register(srv)
	}

	return srv
}

func publicGRPCMethods() []string {
	var public []string

	for _, m := range modules() {
		public = append(public, m.grpcPublic...)
	}

	return public
}
