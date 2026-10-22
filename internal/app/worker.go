package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/m7medvision/go-clean-template/config"
	workerctl "github.com/m7medvision/go-clean-template/internal/controller/worker"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queue"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// RunWorker runs the queue worker: it consumes the queues registered in internal/controller/worker
// until SIGINT/SIGTERM. It does not apply the schema; the app does.
func RunWorker(cfg *config.Config) {
	l := logger.New(cfg.Log.Level)

	ctx := context.Background()

	// Tracing
	shutdownTracing, err := initTracing(ctx, cfg, cfg.App.Name+"-worker")
	if err != nil {
		l.Fatal(fmt.Errorf("app - RunWorker - initTracing: %w", err))
	}
	defer func() {
		if err := shutdownTracing(ctx); err != nil {
			l.Error(fmt.Errorf("app - RunWorker - shutdownTracing: %w", err))
		}
	}()

	// Repository
	pg, err := postgres.New(cfg.PG.URL, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		l.Fatal(fmt.Errorf("app - RunWorker - postgres.New: %w", err))
	}
	defer pg.Close()

	// scaffold:email:begin
	// Use cases, built as the app builds them; the worker's handlers use only some
	uc, err := initUseCases(cfg, pg, jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry))
	if err != nil {
		l.Fatal(fmt.Errorf("app - RunWorker - initUseCases: %w", err))
	}
	// scaffold:email:end

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
		workerctl.WithMail(uc.mail), // scaffold:email
	)

	w.Start()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	sig := <-interrupt
	l.Info("app - RunWorker - signal: %s", sig.String())

	if err = w.Shutdown(); err != nil {
		l.Error(fmt.Errorf("app - RunWorker - worker.Shutdown: %w", err))
	}
}
