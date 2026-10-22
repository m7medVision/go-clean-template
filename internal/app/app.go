// Package app configures and runs application.
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
	"github.com/m7medvision/go-clean-template/internal/repo/notify"
	persistExchangeRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/exchange"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queue"
	persistTaskRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	persistUserRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/user"
	"github.com/m7medvision/go-clean-template/internal/repo/webapi"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/internal/usecase/exchange"
	"github.com/m7medvision/go-clean-template/internal/usecase/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/user"
	"github.com/m7medvision/go-clean-template/pkg/httpserver"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/m7medvision/go-clean-template/pkg/tracing"
)

type useCases struct {
	exchange usecase.Exchange
	user     usecase.User
	task     usecase.Task
	mail     usecase.Mail    // scaffold:email
	passkey  usecase.Passkey // scaffold:passkeys
	oidc     usecase.OIDC    // scaffold:sso
}

// initUseCases builds every use case.
// The worker builds them with the same call. // scaffold:email
// It fails when an optional feature's use case cannot be built from its settings.
func initUseCases(cfg *config.Config, pg *postgres.Postgres, jwtManager *jwt.Manager) (useCases, error) {
	uc := useCases{
		user: user.New(
			persistUserRepo.New(pg),
			notify.New(queue.New(pg), strings.TrimSuffix(cfg.App.BaseURL, "/")+"/login"), // scaffold:welcome
			jwtManager,
		),
		task:     task.New(persistTaskRepo.New(pg)),
		exchange: exchange.New(persistExchangeRepo.New(pg), webapi.New(cfg.Exchange.ServiceURL)),
	}

	deps := featureDeps{cfg: cfg, pg: pg, jwt: jwtManager}

	for _, add := range []addUseCase{
		addMailUseCase,    // scaffold:email
		addPasskeyUseCase, // scaffold:passkeys
		addOIDCUseCase,    // scaffold:sso
	} {
		if err := add(deps, &uc); err != nil {
			return useCases{}, err
		}
	}

	return uc, nil
}

// featureDeps is what an optional feature's use case is built from.
type featureDeps struct {
	cfg *config.Config
	pg  *postgres.Postgres
	jwt *jwt.Manager
}

// addUseCase builds one optional feature's use case into uc. Each lives in the feature's file here.
type addUseCase func(deps featureDeps, uc *useCases) error

func initServers(cfg *config.Config, uc *useCases, jwtManager *jwt.Manager, l logger.Interface) servers {
	router := restapi.NewRouter(
		cfg, uc.exchange, uc.user, uc.task, jwtManager, l,
		restapi.WithPasskeys(uc.passkey), // scaffold:passkeys
		restapi.WithOIDC(uc.oidc),        // scaffold:sso
	)
	mountGraphQL(router, cfg, uc, jwtManager, l) // scaffold:graphql

	httpServer := httpserver.New(
		l,
		httpserver.Port(cfg.HTTP.Port),
		httpserver.Handler(router),
	)

	return servers{
		{name: "httpServer", server: httpServer},
		newGRPCServer(cfg, uc, jwtManager, l), // scaffold:grpc
	}
}

func initTracing(ctx context.Context, cfg *config.Config, serviceName string) (tracing.ShutdownFunc, error) {
	shutdown, err := tracing.New(ctx, tracing.Config{
		Enabled:     cfg.Tracing.Enabled,
		ServiceName: serviceName,
		Version:     cfg.App.Version,
		Endpoint:    cfg.Tracing.OTLPEndpoint,
		Insecure:    cfg.Tracing.OTLPInsecure,
		SampleRate:  cfg.Tracing.SampleRate,
	})
	if err != nil {
		return nil, fmt.Errorf("tracing.New: %w", err)
	}

	return shutdown, nil
}

// Run creates objects via constructors.
func Run(cfg *config.Config) {
	l := logger.New(cfg.Log.Level)

	ctx := context.Background()

	// Tracing
	shutdownTracing, err := initTracing(ctx, cfg, cfg.App.Name)
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - initTracing: %w", err))
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

	uc, err := initUseCases(cfg, pg, jwtManager)
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - initUseCases: %w", err))
	}

	s := initServers(cfg, &uc, jwtManager, l)
	s.start()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	s.waitForShutdown(interrupt, l)
}
