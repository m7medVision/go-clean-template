package middleware

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Metrics collects gRPC call counters and latencies.
type Metrics struct {
	calls     *prometheus.CounterVec
	durations *prometheus.HistogramVec
}

// NewMetrics registers the collectors on reg. Registering them again reuses the ones already
// there, so building several servers in one process (tests) stays safe.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	labels := []string{"method", "code"}

	return &Metrics{
		calls: reuseRegistered(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC calls served.",
		}, labels)),
		durations: reuseRegistered(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "gRPC call latencies in seconds.",
			Buckets: prometheus.DefBuckets,
		}, labels)),
	}
}

// Interceptor counts and times every unary call, labeled by its full method name and status code.
func (m *Metrics) Interceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	labels := []string{info.FullMethod, status.Code(err).String()}
	m.calls.WithLabelValues(labels...).Inc()
	m.durations.WithLabelValues(labels...).Observe(time.Since(start).Seconds())

	return resp, err
}

// reuseRegistered registers c on reg and returns it, or returns the collector of the same name
// already registered. Any other failure is a programming error, and panics.
func reuseRegistered[T prometheus.Collector](reg prometheus.Registerer, c T) T {
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
