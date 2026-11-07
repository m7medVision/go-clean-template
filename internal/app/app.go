// Package app configures and runs application.
package app

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
)

// Run applies the schema, imports what predates event sourcing, and serves HTTP and gRPC until ctx
// ends or a server fails. It returns once both have stopped and the pool is closed.
func Run(ctx context.Context, cfg *config.Config) error {
	b, err := newBootstrap(ctx, cfg, cfg.App.Name)
	if err != nil {
		return fmt.Errorf("app - Run: %w", err)
	}
	defer b.close(ctx)

	if err = syncSchema(ctx, b.l, cfg.PG.URL); err != nil {
		return fmt.Errorf("app - Run - syncSchema: %w", err)
	}

	// Rows written before a domain stored events become its first events, once.
	if err = eventstore.Backfill(ctx, b.pg, importers(b.pg, b.store)...); err != nil {
		return fmt.Errorf("app - Run - eventstore.Backfill: %w", err)
	}

	jwtManager := jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry)

	if err = registerHandlers(cfg, b.buses, b.store, b.pg, jwtManager); err != nil {
		return fmt.Errorf("app - Run - registerHandlers: %w", err)
	}

	// The worker runs the projections; the app serves /metrics, so it reports how far behind they are.
	if cfg.Metrics.Enabled {
		if err = prometheus.DefaultRegisterer.Register(projection.NewLagCollector(b.pg)); err != nil {
			return fmt.Errorf("app - Run - register the projection lag collector: %w", err)
		}
	}

	router := restapi.NewRouter(
		restapi.Settings{
			Version: cfg.App.Version, Docs: cfg.Docs.Enabled, Tracing: cfg.Tracing.Enabled, Metrics: cfg.Metrics.Enabled,
		},
		b.buses, jwtManager, b.l,
		restapi.WithPasskeys(),
		restapi.WithOIDC(),
	)
	mountGraphQL(router, cfg.Docs.Enabled, b.buses, jwtManager, b.l)

	httpServer := newHTTPServer(cfg.HTTP.Port, router, cfg.CQRS.ConsistencyTimeout)
	grpcServer := newGRPCServer(b.buses, jwtManager, b.l)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return serveHTTP(ctx, httpServer, b.l) })
	g.Go(func() error { return serveGRPC(ctx, grpcServer, cfg.GRPC.Port, b.l) })

	if err = g.Wait(); err != nil {
		return fmt.Errorf("app - Run: %w", err)
	}

	return nil
}
