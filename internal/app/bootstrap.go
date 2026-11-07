package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/m7medvision/go-clean-template/pkg/tracing"
)

// bootstrap is what both binaries start from: the logger, tracing, the database, the event store
// and the buses, with no handler registered yet.
type bootstrap struct {
	l           *slog.Logger
	pg          *postgres.Postgres
	store       repo.EventStore
	buses       cqrs.Buses
	stopTracing tracing.ShutdownFunc
}

// newBootstrap starts what the binary called service runs on. close undoes it.
func newBootstrap(ctx context.Context, cfg *config.Config, service string) (*bootstrap, error) {
	l := newLogger(cfg.Log.Level)

	stopTracing, err := tracing.New(ctx, service, cfg.App.Version, tracing.Config(cfg.Tracing))
	if err != nil {
		return nil, fmt.Errorf("tracing.New: %w", err)
	}

	pg, err := postgres.New(cfg.PG.URL, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		if stopErr := stopTracing(context.WithoutCancel(ctx)); stopErr != nil {
			l.Error("app - stop tracing", "error", stopErr)
		}

		return nil, fmt.Errorf("postgres.New: %w", err)
	}

	cmds, qs := newBuses(pg, subscribersOf(projectors(pg)), cfg.CQRS.ConsistencyTimeout, l)

	return &bootstrap{
		l:           l,
		pg:          pg,
		store:       newEventStore(pg, cfg.CQRS.SnapshotEvery),
		buses:       cqrs.Buses{Commands: cmds, Queries: qs},
		stopTracing: stopTracing,
	}, nil
}

// close closes the pool, then flushes the spans still buffered. Call it once everything that uses
// them has returned. It flushes even when ctx has ended.
func (b *bootstrap) close(ctx context.Context) {
	b.pg.Close()

	if err := b.stopTracing(context.WithoutCancel(ctx)); err != nil {
		b.l.Error("app - stop tracing", "error", err)
	}
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
