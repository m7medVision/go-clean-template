// Package app configures and runs application.
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/evrone/go-clean-template/config"
	"github.com/evrone/go-clean-template/internal/controller/graphql"
	"github.com/evrone/go-clean-template/internal/controller/grpc"
	grpcmw "github.com/evrone/go-clean-template/internal/controller/grpc/middleware"
	"github.com/evrone/go-clean-template/internal/controller/restapi"
	persistTaskRepo "github.com/evrone/go-clean-template/internal/repo/persistent/task"
	persistTranslationRepo "github.com/evrone/go-clean-template/internal/repo/persistent/translation"
	persistUserRepo "github.com/evrone/go-clean-template/internal/repo/persistent/user"
	"github.com/evrone/go-clean-template/internal/repo/webapi"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/internal/usecase/task"
	"github.com/evrone/go-clean-template/internal/usecase/translation"
	"github.com/evrone/go-clean-template/internal/usecase/user"
	"github.com/evrone/go-clean-template/pkg/grpcserver"
	"github.com/evrone/go-clean-template/pkg/httpserver"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/evrone/go-clean-template/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	pbgrpc "google.golang.org/grpc"
)

type useCases struct {
	translation usecase.Translation
	user        usecase.User
	task        usecase.Task
}

type servers struct {
	grpc *grpcserver.Server
	http *httpserver.Server
}

func initUseCases(pg *postgres.Postgres, jwtManager *jwt.Manager) useCases {
	translationRepo := persistTranslationRepo.New(pg)
	taskRepo := persistTaskRepo.New(pg)
	userRepo := persistUserRepo.New(pg)

	return useCases{
		user:        user.New(userRepo, jwtManager),
		task:        task.New(taskRepo),
		translation: translation.New(translationRepo, webapi.New()),
	}
}

func initServers(cfg *config.Config, uc useCases, jwtManager *jwt.Manager, l logger.Interface) servers {
	// gRPC Server
	grpcServer := grpcserver.New(
		l,
		grpcserver.Port(cfg.GRPC.Port),
		grpcserver.ServerOptions(
			pbgrpc.UnaryInterceptor(grpcmw.AuthInterceptor(jwtManager)),
			pbgrpc.StatsHandler(otelgrpc.NewServerHandler()),
		),
	)
	grpc.NewRouter(grpcServer.App, uc.translation, uc.user, uc.task, l)

	// HTTP Server
	router := restapi.NewRouter(cfg, uc.translation, uc.user, uc.task, jwtManager, l)
	router.Handle("/graphql", graphql.NewHandler(uc.task, jwtManager, l))

	if cfg.Docs.Enabled {
		router.Handle("/graphql/playground", graphql.NewPlayground("/graphql"))
	}

	httpServer := httpserver.New(
		l,
		httpserver.Port(cfg.HTTP.Port),
		httpserver.Handler(router),
	)

	return servers{
		grpc: grpcServer,
		http: httpServer,
	}
}

func (s *servers) startServers() {
	s.grpc.Start()
	s.http.Start()
}

func (s *servers) waitForShutdown(l logger.Interface) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	var err error

	select {
	case sig := <-interrupt:
		l.Info("app - Run - signal: %s", sig.String())
	case err = <-s.http.Notify():
		l.Error(fmt.Errorf("app - Run - httpServer.Notify: %w", err))
	case err = <-s.grpc.Notify():
		l.Error(fmt.Errorf("app - Run - grpcServer.Notify: %w", err))
	}

	s.shutdownServers(l)
}

func (s *servers) shutdownServers(l logger.Interface) {
	if err := s.http.Shutdown(); err != nil {
		l.Error(fmt.Errorf("app - Run - httpServer.Shutdown: %w", err))
	}

	if err := s.grpc.Shutdown(); err != nil {
		l.Error(fmt.Errorf("app - Run - grpcServer.Shutdown: %w", err))
	}
}

// Run creates objects via constructors.
func Run(cfg *config.Config) {
	l := logger.New(cfg.Log.Level)

	ctx := context.Background()

	// Tracing
	shutdownTracing, err := tracing.New(ctx, tracing.Config{
		Enabled:     cfg.Tracing.Enabled,
		ServiceName: cfg.App.Name,
		Version:     cfg.App.Version,
		Endpoint:    cfg.Tracing.OTLPEndpoint,
		Insecure:    cfg.Tracing.OTLPInsecure,
		SampleRate:  cfg.Tracing.SampleRate,
	})
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - tracing.New: %w", err))
	}
	defer func() {
		if err := shutdownTracing(ctx); err != nil {
			l.Error(fmt.Errorf("app - Run - shutdownTracing: %w", err))
		}
	}()

	syncSchema()

	// Repository
	pg, err := postgres.New(cfg.PG.URL, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.New: %w", err))
	}
	defer pg.Close()

	// JWT
	jwtManager := jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry)

	uc := initUseCases(pg, jwtManager)
	s := initServers(cfg, uc, jwtManager, l)
	s.startServers()
	s.waitForShutdown(l)
}
