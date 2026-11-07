package middleware

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type Metrics struct {
	calls     *prometheus.CounterVec
	durations *prometheus.HistogramVec
}

// NewMetrics registers the call counter and latency histogram on reg. It panics when reg already
// holds them: one registry per server, as the app builds it and as each test does.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	labels := []string{"method", "code"}

	m := &Metrics{
		calls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC calls served.",
		}, labels),
		durations: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "gRPC call latencies in seconds.",
			Buckets: prometheus.DefBuckets,
		}, labels),
	}

	reg.MustRegister(m.calls, m.durations)

	return m
}

func (m *Metrics) Interceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	labels := []string{info.FullMethod, status.Code(err).String()}
	m.calls.WithLabelValues(labels...).Inc()
	m.durations.WithLabelValues(labels...).Observe(time.Since(start).Seconds())

	return resp, err
}
