package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/m7medvision/go-clean-template/config"
	workerv1 "github.com/m7medvision/go-clean-template/internal/controller/worker/v1"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queue"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/m7medvision/go-clean-template/pkg/worker"
	"golang.org/x/sync/errgroup"
)

// RunWorker runs the projections and consumes the mail queue until ctx ends. It returns once both
// have stopped and the pool is closed. It does not apply the schema; the app does.
func RunWorker(ctx context.Context, cfg *config.Config) error {
	b, err := newBootstrap(ctx, cfg, cfg.App.Name+"-worker")
	if err != nil {
		return fmt.Errorf("app - RunWorker: %w", err)
	}
	defer b.close(ctx)

	// Use cases, built as the app builds them; the worker's handlers use only some
	if err = registerHandlers(cfg, b.buses, b.store, b.pg, jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry)); err != nil {
		return fmt.Errorf("app - RunWorker - registerHandlers: %w", err)
	}

	runner, err := newRunner(b.pg, &cfg.CQRS, loginURL(cfg.App.BaseURL), b.l)
	if err != nil {
		return fmt.Errorf("app - RunWorker - newRunner: %w", err)
	}

	w := worker.New(b.l, queue.New(b.pg), worker.Settings(cfg.Worker))
	workerv1.NewMailRoutes(w, b.buses.Commands, b.l)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.Run(ctx) })
	g.Go(func() error { return runner.Run(ctx) })

	if err = g.Wait(); err != nil {
		return fmt.Errorf("app - RunWorker: %w", err)
	}

	return nil
}

// newRunner returns the runner of every subscriber. The welcome email links to loginURL.
func newRunner(pg *postgres.Postgres, s *config.CQRS, loginURL string, l *slog.Logger) (*projection.Runner, error) {
	ps, handlers := projectors(pg), eventHandlers(pg, loginURL)
	if err := projection.Check(ps, handlers, entity.EventTypes()); err != nil {
		return nil, err
	}

	store := projection.NewStore(pg, eventstore.NewReader(pg, newCodec()), l)
	runner := projection.New(l, store, s.ProjectionPollInterval, s.ProjectionBatchSize)

	for _, p := range ps {
		runner.Add(p)
	}

	for _, h := range handlers {
		runner.Add(h)
	}

	return runner, nil
}

// RebuildProjection empties the read model of the projector called name and resets its checkpoint,
// for the worker that runs it to replay every event into it (make projection-rebuild).
func RebuildProjection(ctx context.Context, cfg *config.Config, name string) error {
	pg, err := postgres.New(cfg.PG.URL, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		return fmt.Errorf("app - RebuildProjection - postgres.New: %w", err)
	}
	defer pg.Close()

	if err = projection.Rebuild(ctx, pg, name, projectors(pg)...); err != nil {
		return fmt.Errorf("app - RebuildProjection: %w", err)
	}

	return nil
}
