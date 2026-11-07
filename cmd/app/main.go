package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx)

	stop()

	if err != nil {
		slog.Error("app stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	return app.Run(ctx, cfg)
}
