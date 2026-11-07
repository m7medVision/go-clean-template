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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// bootstrap is what both binaries start from: the logger, tracing, the metrics registry, the
// database, the event store and the buses, with no handler registered yet.
type bootstrap struct {
	l           *slog.Logger
	pg          *postgres.Postgres
	store       repo.EventStore
	buses       cqrs.Buses
	metrics     *prometheus.Registry
	stopTracing tracing.ShutdownFunc
}

// newBootstrap starts what the binary called service runs on. close undoes it.
func newBootstrap(ctx context.Context, cfg *config.Config, service string) (*bootstrap, error) {
	l := newLogger(cfg.Log.Level)

	stopTracing, err := tracing.New(ctx, service, cfg.App.Version, tracing.Config(cfg.Tracing))
	if err != nil {
		return nil, fmt.Errorf("tracing.New: %w", err)
	}

	pg, err := openPool(ctx, cfg.PG)
	if err != nil {
		if stopErr := stopTracing(context.WithoutCancel(ctx)); stopErr != nil {
			l.Error("app - stop tracing", "error", stopErr)
		}

		return nil, err
	}

	metrics := newMetrics()
	cmds, qs := newBuses(pg, projectors(pg), cfg.CQRS.ConsistencyTimeout, metrics, l)

	return &bootstrap{
		l:           l,
		pg:          pg,
		store:       newEventStore(pg, cfg.CQRS.SnapshotEvery),
		buses:       cqrs.Buses{Commands: cmds, Queries: qs},
		metrics:     metrics,
		stopTracing: stopTracing,
	}, nil
}

// openPool connects to the database, waiting for it until ctx ends: both binaries and the rebuild
// command open their pool here.
func openPool(ctx context.Context, cfg config.PG) (*postgres.Postgres, error) {
	pg, err := postgres.New(ctx, cfg.URL, cfg.PoolMax)
	if err != nil {
		return nil, fmt.Errorf("postgres.New: %w", err)
	}

	return pg, nil
}

// newMetrics is the registry every collector registers on: the buses, both transports and the
// projection lag. It starts with the Go runtime and process collectors Prometheus's default registry
// holds, so /metrics serves what promhttp.Handler would. Whether anything serves it is /metrics'
// choice alone (METRICS_ENABLED): the collectors count either way.
func newMetrics() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), collectors.NewGoCollector())

	return reg
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
