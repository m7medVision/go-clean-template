//go:build migrate

package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/m7medvision/go-clean-template/internal/app/migrations"
	"github.com/m7medvision/go-clean-template/pkg/migrate"
)

const _migrateTimeout = 5 * time.Minute

func migrateSchema(ctx context.Context, l *slog.Logger, databaseURL string) error {
	ctx, cancel := context.WithTimeout(ctx, _migrateTimeout)
	defer cancel()

	m, err := migrate.Open(ctx, databaseURL, migrations.FS, l)
	if err != nil {
		return err
	}
	defer m.Close()

	if upErr := m.Up(ctx); upErr != nil {
		return upErr
	}

	l.Info("migrate - schema up to date")

	return nil
}
