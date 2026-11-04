package cqrs

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	_outcomeOK    = "ok"
	_outcomeError = "error"
)

// Metrics counts every message by bus, name and outcome (cqrs_messages_total) and times it
// (cqrs_message_duration_seconds). Registering on a registerer that already has the collectors
// reuses them, so building several buses in one process stays safe.
func Metrics(reg prometheus.Registerer) Step {
	labels := []string{"kind", "name", "outcome"}

	messages := registerCollector(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cqrs_messages_total",
		Help: "Commands and queries handled, by bus, message and outcome.",
	}, labels))

	durations := registerCollector(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cqrs_message_duration_seconds",
		Help:    "Time to handle a command or query, by bus, message and outcome.",
		Buckets: prometheus.DefBuckets,
	}, labels))

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

// registerCollector returns c registered on reg, or the collector already registered under its name.
func registerCollector[T prometheus.Collector](reg prometheus.Registerer, c T) T {
	err := reg.Register(c)
	if err == nil {
		return c
	}

	if already, ok := errors.AsType[prometheus.AlreadyRegisteredError](err); ok {
		if existing, isType := already.ExistingCollector.(T); isType {
			return existing
		}
	}

	panic(err)
}
