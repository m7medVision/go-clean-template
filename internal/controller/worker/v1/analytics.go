package v1

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/m7medvision/go-clean-template/internal/entity/analytics"
	"github.com/m7medvision/go-clean-template/internal/usecase/analytics/command"
	"github.com/m7medvision/go-clean-template/pkg/broker"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

type AnalyticsController struct {
	cmds *cqrs.Commands
	sink string
	l    *slog.Logger
}

func NewAnalyticsController(cmds *cqrs.Commands, sink string, l *slog.Logger) *AnalyticsController {
	return &AnalyticsController{cmds: cmds, sink: sink, l: l}
}

// NewAnalyticsRoutes registers one topic per sink, so a sink that fails retries alone.
func NewAnalyticsRoutes(r Router, cmds *cqrs.Commands, sinks []string, l *slog.Logger) {
	for _, sink := range sinks {
		r.Handle(analytics.QueuePrefix+sink, NewAnalyticsController(cmds, sink, l).Handle)
	}
}

// Handle delivers one analytics event to the controller's sink. A body that does not decode, a sink
// this worker does not have and an event the vendor refuses never succeed: they go to the poison
// topic without retries.
func (c *AnalyticsController) Handle(ctx context.Context, body []byte) error {
	e, err := analytics.Decode(body)
	if err != nil {
		return broker.Permanent(fmt.Errorf("worker - v1 - analytics - decode: %w", err))
	}

	_, err = cqrs.SendCommand(ctx, c.cmds, command.DeliverAnalytics{Sink: c.sink, Event: e})
	if errors.Is(err, analytics.ErrUnknownSink) || errors.Is(err, analytics.ErrRejected) {
		return broker.Permanent(fmt.Errorf("worker - v1 - analytics - deliver: %w", err))
	}

	if err != nil {
		return fmt.Errorf("worker - v1 - analytics - DeliverAnalytics: %w", err)
	}

	return nil
}
