//go:build !migrate

package app

import (
	"context"
	"log/slog"
)

func migrateSchema(context.Context, *slog.Logger, string) error { return nil }
