//go:build !migrate

package app

import (
	"context"
	"log/slog"
)

func syncSchema(context.Context, *slog.Logger, string) error { return nil }
