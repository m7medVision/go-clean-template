// Package app configures and runs application.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/httpserver"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/m7medvision/go-clean-template/pkg/tracing"
	"github.com/prometheus/client_golang/prometheus"
)

type useCases struct {
	// buses are the command and query buses: every domain registers its handlers on them, and the
	// transports and the worker send through them.
	buses cqrs.Buses
}

// initUseCases builds the buses and registers every domain's handlers on them.
// The worker builds them with the same call.
// It fails when a handler cannot be built from its settings, such as a broken email template.
func initUseCases(
	cfg *config.Config, pg *postgres.Postgres, jwtManager *jwt.Manager, l *slog.Logger,
) (useCases, error) {
	cmds, qs := newBuses(cfg, pg, subscribersOf(projectors(pg)), l)
	if err := registerHandlers(cfg, cmds, qs, newEventStore(cfg, pg), pg, jwtManager); err != nil {
		return useCases{}, err
	}

	return useCases{buses: cqrs.Buses{Commands: cmds, Queries: qs}}, nil
}

func initServers(cfg *config.Config, uc *useCases, jwtManager *jwt.Manager, l *slog.Logger) servers {
	router := restapi.NewRouter(
		cfg, uc.buses, jwtManager, l,
		restapi.WithPasskeys(),
		restapi.WithOIDC(),
	)
	mountGraphQL(router, cfg, uc, jwtManager, l)

	httpServer := httpserver.New(
		l,
		httpserver.Port(cfg.HTTP.Port),
		httpserver.Handler(router),
	)

	return servers{
		{name: "httpServer", server: httpServer},
		newGRPCServer(cfg, uc, jwtManager, l),
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

// Run serves the app until SIGINT/SIGTERM or a server fails.
func Run(cfg *config.Config) error {
	l := newLogger(cfg.Log.Level)

	ctx := context.Background()

	// Tracing
	shutdownTracing, err := initTracing(ctx, cfg, cfg.App.Name)
	if err != nil {
		return fmt.Errorf("app - Run - initTracing: %w", err)
	}
	defer func() {
		if err := shutdownTracing(ctx); err != nil {
			l.Error("app - Run - shutdownTracing", "error", err)
		}
	}()

	// Repository
	pg, err := postgres.New(cfg.PG.URL, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		return fmt.Errorf("app - Run - postgres.New: %w", err)
	}
	defer pg.Close()

	if err = syncSchema(ctx, l, cfg.PG.URL); err != nil {
		return fmt.Errorf("app - Run - syncSchema: %w", err)
	}

	// Rows written before a domain stored events become its first events, once.
	if err = eventstore.Backfill(ctx, pg, newEventStore(cfg, pg), importers(pg)...); err != nil {
		return fmt.Errorf("app - Run - eventstore.Backfill: %w", err)
	}

	// JWT
	jwtManager := jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry)

	uc, err := initUseCases(cfg, pg, jwtManager, l)
	if err != nil {
		return fmt.Errorf("app - Run - initUseCases: %w", err)
	}

	// The worker runs the projections; the app serves /metrics, so it reports how far behind they are.
	if cfg.Metrics.Enabled {
		if err = prometheus.DefaultRegisterer.Register(projection.NewLagCollector(pg)); err != nil {
			return fmt.Errorf("app - Run - register the projection lag collector: %w", err)
		}
	}

	s := initServers(cfg, &uc, jwtManager, l)
	s.start()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	s.waitForShutdown(interrupt, l)

	return nil
}

// newLogger writes JSON lines to stdout at the level named by LOG_LEVEL (debug, info, warn or
// error); any other name is info.
func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
