// Package app configures and runs application.
package app

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
)

// Run applies the schema, imports what predates event sourcing, and serves HTTP and gRPC until ctx
// ends or a server fails. It returns once both have stopped and the pool is closed. It installs every
// module's handlers but mail delivery's, which only the worker runs.
func Run(ctx context.Context, cfg *config.Config) error {
	b, err := newBootstrap(ctx, cfg, cfg.App.Name)
	if err != nil {
		return fmt.Errorf("app - Run: %w", err)
	}
	defer b.close(ctx)

	if err = syncSchema(ctx, b.l, cfg.PG.URL); err != nil {
		return fmt.Errorf("app - Run - syncSchema: %w", err)
	}

	d := &deps{cfg: cfg, pg: b.pg, store: b.store, l: b.l, tokens: jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry)}

	if err = eventstore.Backfill(ctx, b.pg, b.store, importers(d)...); err != nil {
		return fmt.Errorf("app - Run - eventstore.Backfill: %w", err)
	}

	if err = registerHandlers(d, b.buses); err != nil {
		return fmt.Errorf("app - Run - registerHandlers: %w", err)
	}

	// The worker runs the projections; the app serves /metrics, so it reports how far behind they are.
	if cfg.Metrics.Enabled {
		if err = prometheus.DefaultRegisterer.Register(projection.NewLagCollector(b.pg)); err != nil {
			return fmt.Errorf("app - Run - register the projection lag collector: %w", err)
		}
	}

	httpServer := newHTTPServer(cfg.HTTP.Port, newHTTPHandler(cfg, b.buses, d.tokens, b.l), cfg.CQRS.ConsistencyTimeout)
	grpcServer := newGRPCServer(cfg, b.buses, d.tokens, b.l)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return serveHTTP(ctx, httpServer, b.l) })
	g.Go(func() error { return serveGRPC(ctx, grpcServer, cfg.GRPC.Port, b.l) })

	if err = g.Wait(); err != nil {
		return fmt.Errorf("app - Run: %w", err)
	}

	return nil
}
