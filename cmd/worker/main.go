package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/app"
)

func main() {
	rebuild := flag.String("rebuild", "", "empty the named projection's read model for the running worker to replay, then exit")

	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, *rebuild)

	stop()

	if err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

// run runs the worker until SIGINT or SIGTERM, or, given a projection to rebuild, resets it and
// returns.
func run(ctx context.Context, rebuild string) error {
	cfg, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if rebuild == "" {
		return app.RunWorker(ctx, cfg)
	}

	if err := app.RebuildProjection(ctx, cfg, rebuild); err != nil {
		return err
	}

	slog.Info("projection reset: the running worker replays it from the first event", "projection", rebuild)

	return nil
}
