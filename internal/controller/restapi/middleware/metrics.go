package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics collects HTTP request counters and latencies.
type Metrics struct {
	requests  *prometheus.CounterVec
	durations *prometheus.HistogramVec
}

// NewMetrics registers the collectors on the given registerer. Registering
// again for an already used set reuses the existing collector, so building
// several handlers in one process (tests) stays safe.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests served.",
	}, []string{"code"})

	durations := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latencies in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"code"})

	requests = registerMetric(reg, requests)
	durations = registerMetric(reg, durations)

	return &Metrics{
		requests:  requests,
		durations: durations,
	}
}

// registerMetric returns the registered collector, keeping the pre-existing
// one when the name was already taken.
func registerMetric[T prometheus.Collector](reg prometheus.Registerer, c T) T {
	err := reg.Register(c)
	if err == nil {
		return c
	}

	if already, ok := errors.AsType[prometheus.AlreadyRegisteredError](err); ok {
		existing, isType := already.ExistingCollector.(T)
		if !isType {
			panic(err)
		}

		return existing
	}

	panic(err)
}

// Middleware instruments every request with the request counter and the
// latency histogram.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		code := strconv.Itoa(rec.status)
		m.requests.WithLabelValues(code).Inc()
		m.durations.WithLabelValues(code).Observe(time.Since(start).Seconds())
	})
}
