package cqrs

import (
	"context"
	"time"

	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// Log writes one debug line per message: its bus and name, its outcome and how long it took, and
// the error when it failed. Controllers still log the failures they answer with a 500.
func Log(l logger.Interface) Step {
	return func(ctx context.Context, msg Message, next Next) (any, error) {
		start := time.Now()

		out, err := next(ctx)
		if err != nil {
			l.Debug("cqrs - %s %s - error in %s: %v", msg.Kind, msg.Name, time.Since(start), err)

			return out, err
		}

		l.Debug("cqrs - %s %s - ok in %s", msg.Kind, msg.Name, time.Since(start))

		return out, nil
	}
}
