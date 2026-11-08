// Package app configures and runs application.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/limiter"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/usecase/behavior"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/valkey"
	"golang.org/x/sync/errgroup"
)

// ErrNoJWTSecret is a JWT_SECRET left unset. Only the app needs one: config does not require it, so
// the worker starts without it, and Run refuses to start without it.
var ErrNoJWTSecret = errors.New("app: JWT_SECRET is not set")

var ErrNoValkeyURL = errors.New("app: VALKEY_URL is not set")

// Run applies the schema, imports what predates event sourcing, and serves HTTP and gRPC until ctx
// ends or a server fails. It returns once both have stopped and the pool is closed. It installs every
// module's handlers but mail delivery's, which only the worker runs.
func Run(ctx context.Context, cfg *config.Config) error {
	proxy, err := appSettings(cfg)
	if err != nil {
		return fmt.Errorf("app - Run: %w", err)
	}

	b, err := newBootstrap(ctx, cfg, cfg.App.Name)
	if err != nil {
		return fmt.Errorf("app - Run: %w", err)
	}
	defer b.close(ctx)

	client, err := valkey.New(ctx, cfg.Valkey.URL)
	if err != nil {
		return fmt.Errorf("app - Run - valkey.New: %w", err)
	}
	defer client.Close()

	lim, err := limiter.New(client, entity.Limits(cfg.AuthLimit))
	if err != nil {
		return fmt.Errorf("app - Run - limiter.New: %w", err)
	}

	buses := newBuses(b.pg, projectors(b.pg), cfg.CQRS.ConsistencyTimeout, b.metrics, b.l, behavior.Limit(lim, b.l))

	if err = syncSchema(ctx, b.l, cfg.PG.URL); err != nil {
		return fmt.Errorf("app - Run - syncSchema: %w", err)
	}

	store := newEventStore(b.pg, cfg.CQRS.SnapshotEvery)
	d := &deps{cfg: cfg, pg: b.pg, store: store, l: b.l, tokens: jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry), valkey: client}

	if err = eventstore.Backfill(ctx, b.pg, store, importers(b.pg)...); err != nil {
		return fmt.Errorf("app - Run - eventstore.Backfill: %w", err)
	}

	if err = registerHandlers(d, buses); err != nil {
		return fmt.Errorf("app - Run - registerHandlers: %w", err)
	}

	// The worker runs the projections; the app serves /metrics, so it reports how far behind they are.
	if err = b.metrics.Register(projection.NewLagCollector(b.pg)); err != nil {
		return fmt.Errorf("app - Run - register the projection lag collector: %w", err)
	}

	httpServer := newHTTPServer(cfg.HTTP.Port, newHTTPHandler(cfg, proxy, buses, d.tokens, b.metrics, b.l), cfg.CQRS.ConsistencyTimeout)
	grpcServer := newGRPCServer(proxy, buses, d.tokens, b.metrics, cfg.Docs.Enabled, b.l)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return serveHTTP(ctx, httpServer, _shutdownTimeout, b.l) })
	g.Go(func() error { return serveGRPC(ctx, grpcServer, cfg.GRPC.Port, _shutdownTimeout, b.l) })

	if err = g.Wait(); err != nil {
		return fmt.Errorf("app - Run: %w", err)
	}

	return nil
}

func appSettings(cfg *config.Config) (netip.Addr, error) {
	if cfg.JWT.Secret == "" {
		return netip.Addr{}, ErrNoJWTSecret
	}

	if cfg.Valkey.URL == "" {
		return netip.Addr{}, ErrNoValkeyURL
	}

	proxy, err := trustedProxy(cfg.HTTP.TrustedProxy)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("TRUSTED_PROXY: %w", err)
	}

	return proxy, nil
}

func trustedProxy(addr string) (netip.Addr, error) {
	if addr == "" {
		return netip.Addr{}, nil
	}

	return netip.ParseAddr(addr)
}
