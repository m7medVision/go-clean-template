// Package analytics publishes analytics events for the worker to send, one message per sink.
package analytics

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/notify"
)

// Publisher publishes each event once per sink, on the sink's own topic, so a sink that fails is
// retried alone. The message ID is the sink's and the event's: a repeat is dropped by the broker.
type Publisher struct {
	pub   notify.Publisher
	sinks []string
}

var _ repo.Analytics = (*Publisher)(nil)

func New(p notify.Publisher, sinks []string) repo.Analytics {
	return &Publisher{pub: p, sinks: sinks}
}

func (p *Publisher) Capture(ctx context.Context, e *analytics.Event) error {
	body, err := analytics.Encode(e)
	if err != nil {
		return fmt.Errorf("analytics - Capture - analytics.Encode: %w", err)
	}

	for _, sink := range p.sinks {
		if err = p.pub.Publish(ctx, analytics.QueuePrefix+sink, sink+"-"+e.ID, body); err != nil {
			return fmt.Errorf("analytics - Capture - p.pub.Publish %s: %w", sink, err)
		}
	}

	return nil
}
