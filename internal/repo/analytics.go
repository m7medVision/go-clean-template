package repo

import (
	"context"

	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
)

//go:generate mockgen -source=analytics.go -destination=repomock/analytics.go -package=repomock -exclude_interfaces=AnalyticsSink

type (
	// Analytics hands an event to every configured analytics sink, through the queue.
	Analytics interface {
		Capture(ctx context.Context, e *analytics.Event) error
	}

	// AnalyticsSink is one analytics vendor; each implementation is the only code that knows it.
	AnalyticsSink interface {
		Name() string
		Send(ctx context.Context, e *analytics.Event) error
	}
)
