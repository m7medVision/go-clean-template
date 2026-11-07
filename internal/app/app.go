// Package app configures and runs application.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"golang.org/x/sync/errgroup"
)

// The settings only the app needs. config does not require them, so the worker starts without them;
// Run refuses to start without them.
var (
	ErrNoHTTPPort  = errors.New("app: HTTP_PORT is not set")
	ErrNoJWTSecret = errors.New("app: JWT_SECRET is not set")
)

// Run applies the schema, imports what predates event sourcing, and serves HTTP and gRPC until ctx
// ends or a server fails. It returns once both have stopped and the pool is closed. It installs every
// module's handlers but mail delivery's, which only the worker runs.
func Run(ctx context.Context, cfg *config.Config) error {
	if err := checkAppSettings(cfg); err != nil {
		return fmt.Errorf("app - Run: %w", err)
	}

	b, err := newBootstrap(ctx, cfg, cfg.App.Name)
	if err != nil {
		return fmt.Errorf("app - Run: %w", err)
	}
	defer b.close(ctx)

	if err = syncSchema(ctx, b.l, cfg.PG.URL); err != nil {
		return fmt.Errorf("app - Run - syncSchema: %w", err)
	}

	store := newEventStore(b.pg, cfg.CQRS.SnapshotEvery)
	d := &deps{cfg: cfg, pg: b.pg, store: store, l: b.l, tokens: jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry)}

	if err = eventstore.Backfill(ctx, b.pg, store, importers(b.pg)...); err != nil {
		return fmt.Errorf("app - Run - eventstore.Backfill: %w", err)
	}

	if err = registerHandlers(d, b.buses); err != nil {
		return fmt.Errorf("app - Run - registerHandlers: %w", err)
	}

	// The worker runs the projections; the app serves /metrics, so it reports how far behind they are.
	if err = b.metrics.Register(projection.NewLagCollector(b.pg)); err != nil {
		return fmt.Errorf("app - Run - register the projection lag collector: %w", err)
	}

	httpServer := newHTTPServer(cfg.HTTP.Port, newHTTPHandler(cfg, b.buses, d.tokens, b.metrics, b.l), cfg.CQRS.ConsistencyTimeout)
	grpcServer := newGRPCServer(b.buses, d.tokens, b.metrics, b.l)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return serveHTTP(ctx, httpServer, _shutdownTimeout, b.l) })
	g.Go(func() error { return serveGRPC(ctx, grpcServer, cfg.GRPC.Port, _shutdownTimeout, b.l) })

	if err = g.Wait(); err != nil {
		return fmt.Errorf("app - Run: %w", err)
	}

	return nil
}

// checkAppSettings fails on each setting the app needs that config leaves optional for the worker.
func checkAppSettings(cfg *config.Config) error {
	var errs []error

	if cfg.HTTP.Port == "" {
		errs = append(errs, ErrNoHTTPPort)
	}

	if cfg.JWT.Secret == "" {
		errs = append(errs, ErrNoJWTSecret)
	}

	return errors.Join(errs...)
}
