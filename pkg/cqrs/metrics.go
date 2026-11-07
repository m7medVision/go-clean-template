package cqrs

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	_outcomeOK    = "ok"
	_outcomeError = "error"
)

// Metrics counts every message by bus, name and outcome (cqrs_messages_total) and times it
// (cqrs_message_duration_seconds). It registers both on reg, so build it once and give the same
// step to both buses.
func Metrics(reg prometheus.Registerer) Step {
	labels := []string{"kind", "name", "outcome"}

	messages := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cqrs_messages_total",
		Help: "Commands and queries handled, by bus, message and outcome.",
	}, labels)

	durations := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cqrs_message_duration_seconds",
		Help:    "Time to handle a command or query, by bus, message and outcome.",
		Buckets: prometheus.DefBuckets,
	}, labels)

	reg.MustRegister(messages, durations)

	return func(ctx context.Context, msg Message, next Next) (any, error) {
		start := time.Now()

		out, err := next(ctx)

		outcome := _outcomeOK
		if err != nil {
			outcome = _outcomeError
		}

		messages.WithLabelValues(string(msg.Kind), msg.Name, outcome).Inc()
		durations.WithLabelValues(string(msg.Kind), msg.Name, outcome).Observe(time.Since(start).Seconds())

		return out, err
	}
}
