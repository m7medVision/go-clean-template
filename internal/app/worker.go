package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/m7medvision/go-clean-template/config"
	workerctl "github.com/m7medvision/go-clean-template/internal/controller/worker"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queue"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// RunWorker runs the queue worker: it consumes the queues registered in internal/controller/worker
// until SIGINT/SIGTERM. It does not apply the schema; the app does.
func RunWorker(cfg *config.Config) error {
	l := newLogger(cfg.Log.Level)

	ctx := context.Background()

	// Tracing
	shutdownTracing, err := initTracing(ctx, cfg, cfg.App.Name+"-worker")
	if err != nil {
		return fmt.Errorf("app - RunWorker - initTracing: %w", err)
	}
	defer func() {
		if err := shutdownTracing(ctx); err != nil {
			l.Error("app - RunWorker - shutdownTracing", "error", err)
		}
	}()

	// Repository
	pg, err := postgres.New(cfg.PG.URL, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		return fmt.Errorf("app - RunWorker - postgres.New: %w", err)
	}
	defer pg.Close()

	// Use cases, built as the app builds them; the worker's handlers use only some
	uc, err := initUseCases(cfg, pg, jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry), l)
	if err != nil {
		return fmt.Errorf("app - RunWorker - initUseCases: %w", err)
	}

	// Projections and event handlers
	runner, store, err := newRunner(cfg, pg, l)
	if err != nil {
		return fmt.Errorf("app - RunWorker - newRunner: %w", err)
	}

	// Worker
	w := worker.New(
		l,
		queue.New(pg),
		worker.PollInterval(cfg.Worker.PollInterval),
		worker.VisibilityTimeout(cfg.Worker.VisibilityTimeout),
		worker.BatchSize(cfg.Worker.BatchSize),
		worker.ShutdownTimeout(cfg.Worker.ShutdownTimeout),
	)
	workerctl.NewRouter(
		w, l,
		workerctl.WithMail(uc.buses.Commands),
	)

	w.Start()
	runner.Start()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	sig := <-interrupt
	l.Info("app - RunWorker - signal", "signal", sig.String())

	if err = w.Shutdown(); err != nil {
		l.Error("app - RunWorker - worker.Shutdown", "error", err)
	}

	runner.Shutdown()
	store.Close()

	return nil
}

// newRunner returns the runner of every subscriber, and the store it reads, which must be closed
// after the runner shuts down.
func newRunner(cfg *config.Config, pg *postgres.Postgres, l *slog.Logger) (
	*projection.Runner, *projection.PostgresStore, error,
) {
	ps, handlers := projectors(pg), eventHandlers(cfg, pg)
	if err := projection.Check(ps, handlers, entity.EventTypes()); err != nil {
		return nil, nil, err
	}

	store := projection.NewStore(pg, eventstore.NewReader(pg, newCodec()), l)
	runner := projection.New(l, store,
		projection.PollInterval(cfg.CQRS.ProjectionPollInterval),
		projection.BatchSize(cfg.CQRS.ProjectionBatchSize),
	)

	for _, p := range ps {
		runner.Add(p)
	}

	for _, h := range handlers {
		runner.Add(h)
	}

	return runner, store, nil
}

// RebuildProjection empties the read model of the projector called name and resets its checkpoint,
// for the worker that runs it to replay every event into it (make projection-rebuild).
func RebuildProjection(cfg *config.Config, name string) error {
	pg, err := postgres.New(cfg.PG.URL, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		return fmt.Errorf("app - RebuildProjection - postgres.New: %w", err)
	}
	defer pg.Close()

	if err = projection.Rebuild(context.Background(), pg, name, projectors(pg), importers(pg)); err != nil {
		return fmt.Errorf("app - RebuildProjection: %w", err)
	}

	return nil
}
