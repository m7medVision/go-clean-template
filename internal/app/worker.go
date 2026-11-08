package app

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queue"
	"github.com/m7medvision/go-clean-template/pkg/worker"
	"golang.org/x/sync/errgroup"
)

// RunWorker runs every module's projectors and event handlers, and consumes the queues the modules
// register, until ctx ends. It returns once both have stopped and the pool is closed. It installs
// only what its queues send to, so the sign-in settings the app needs do not stop it. It does not
// apply the schema; the app does.
func RunWorker(ctx context.Context, cfg *config.Config) error {
	b, err := newBootstrap(ctx, cfg, cfg.App.Name+"-worker")
	if err != nil {
		return fmt.Errorf("app - RunWorker: %w", err)
	}
	defer b.close(ctx)

	d := &deps{cfg: cfg, pg: b.pg, l: b.l}

	runner, err := newRunner(d)
	if err != nil {
		return fmt.Errorf("app - RunWorker - newRunner: %w", err)
	}

	buses := newBuses(b.pg, projectors(b.pg), cfg.CQRS.ConsistencyTimeout, b.metrics, b.l)

	w := worker.New(b.l, queue.New(b.pg), worker.Settings(cfg.Worker))
	if err = registerQueues(d, buses, w); err != nil {
		return fmt.Errorf("app - RunWorker - registerQueues: %w", err)
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.Run(ctx) })
	g.Go(func() error { return runner.Run(ctx) })

	if err = g.Wait(); err != nil {
		return fmt.Errorf("app - RunWorker: %w", err)
	}

	return nil
}

func newRunner(d *deps) (*projection.Runner, error) {
	ps, handlers := projectors(d.pg), eventHandlers(d)
	if err := projection.Check(ps, handlers, entity.EventTypes()); err != nil {
		return nil, err
	}

	subscribers := make([]repo.Subscriber, 0, len(ps)+len(handlers))
	for _, p := range ps {
		subscribers = append(subscribers, p)
	}

	store := projection.NewStore(d.pg, eventstore.NewReader(d.pg, newCodec()), d.l)

	return projection.New(d.l, store, d.cfg.CQRS.ProjectionPollInterval, d.cfg.CQRS.ProjectionBatchSize,
		append(subscribers, handlers...)...), nil
}

// RebuildProjection empties the read model of the projector called name and resets its checkpoint,
// for the worker that runs it to replay every event into it (make projection-rebuild).
func RebuildProjection(ctx context.Context, cfg *config.Config, name string) error {
	pg, err := openPool(ctx, cfg.PG)
	if err != nil {
		return fmt.Errorf("app - RebuildProjection: %w", err)
	}
	defer pg.Close()

	if err = projection.Rebuild(ctx, pg, name, projectors(pg), importers(pg)); err != nil {
		return fmt.Errorf("app - RebuildProjection: %w", err)
	}

	return nil
}
