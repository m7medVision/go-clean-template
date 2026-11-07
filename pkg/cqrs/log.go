package cqrs

import (
	"context"
	"log/slog"
	"time"
)

// Log writes one debug line per message: its bus and name, its outcome and how long it took, and
// the error when it failed. Controllers still log the failures they answer with a 500.
func Log(l *slog.Logger) Step {
	return func(ctx context.Context, msg Message, next Next) (any, error) {
		start := time.Now()

		out, err := next(ctx)
		if err != nil {
			l.Debug("cqrs - message failed", "kind", msg.Kind, "name", msg.Name,
				"duration", time.Since(start), "error", err)

			return out, err
		}

		l.Debug("cqrs - message handled", "kind", msg.Kind, "name", msg.Name, "duration", time.Since(start))

		return out, nil
	}
}
