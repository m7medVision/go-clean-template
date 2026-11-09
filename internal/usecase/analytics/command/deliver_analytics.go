// Package command holds the analytics domain's command, with its handler.
package command

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// DeliverAnalytics sends Event to the sink called Sink. Only the worker sends it, for each message
// on that sink's topic. Nothing is stored: the delivery is the side effect. A sink this worker does
// not have is analytics.ErrUnknownSink, and an event the vendor refuses analytics.ErrRejected:
// neither goes away on a retry.
type DeliverAnalytics struct {
	cqrs.Command[struct{}]

	Sink  string
	Event *analytics.Event
}

type DeliverAnalyticsHandler struct {
	Sinks map[string]repo.AnalyticsSink
}

func (h DeliverAnalyticsHandler) Handle(ctx context.Context, cmd DeliverAnalytics) (struct{}, error) {
	sink, ok := h.Sinks[cmd.Sink]
	if !ok {
		return struct{}{}, fmt.Errorf("DeliverAnalytics - %q: %w", cmd.Sink, analytics.ErrUnknownSink)
	}

	if err := sink.Send(ctx, cmd.Event); err != nil {
		return struct{}{}, fmt.Errorf("DeliverAnalytics - sink.Send %s: %w", cmd.Sink, err)
	}

	return struct{}{}, nil
}
