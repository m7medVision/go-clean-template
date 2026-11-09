package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/getsentry/sentry-go"
	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/m7medvision/go-clean-template/pkg/sentrylog"
	"github.com/m7medvision/go-clean-template/pkg/tracing"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// bootstrap is what both binaries start from: the logger, tracing, the metrics registry and the
// database. Each binary builds its own buses on them (newBuses), with the steps only it runs.
type bootstrap struct {
	l           *slog.Logger
	pg          *postgres.Postgres
	metrics     *prometheus.Registry
	stopTracing tracing.ShutdownFunc
	flush       func()
}

// newBootstrap starts what the binary called service runs on. close undoes it.
func newBootstrap(ctx context.Context, cfg *config.Config, service string) (*bootstrap, error) {
	l := newLogger(cfg.Log.Level)

	l, flush, err := errorTracking(l, cfg, service)
	if err != nil {
		return nil, err
	}

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

	return &bootstrap{
		l:           l,
		pg:          pg,
		metrics:     newMetrics(),
		stopTracing: stopTracing,
		flush:       flush,
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

	b.flush()
}

// errorTracking wraps l to report its Error records to the service's GlitchTip project when
// SENTRY_DSN is set; without it l is returned as it is.
func errorTracking(l *slog.Logger, cfg *config.Config, service string) (*slog.Logger, func(), error) {
	if cfg.Sentry.DSN == "" {
		return l, func() {}, nil
	}

	flush, err := sentrylog.Init(sentrylog.Options{
		DSN: cfg.Sentry.DSN, Environment: cfg.Sentry.Environment, Release: cfg.App.Version, Service: service,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("app - errorTracking: %w", err)
	}

	return slog.New(sentrylog.New(l.Handler(), sentry.CurrentHub())), flush, nil
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
