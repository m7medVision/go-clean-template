package httpmw

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const _unmatchedRoute = "unmatched"

type Metrics struct {
	requests  *prometheus.CounterVec
	durations *prometheus.HistogramVec
}

// NewMetrics registers the request counter and latency histogram on reg. It panics when reg already
// holds them: one registry per handler, as the app builds it and as each test does.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	labels := []string{"method", "route", "code"}

	m := &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests served.",
		}, labels),
		durations: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latencies in seconds.",
			Buckets: prometheus.DefBuckets,
		}, labels),
	}

	reg.MustRegister(m.requests, m.durations)

	return m
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			route := routePattern(r)
			if route == "" {
				route = _unmatchedRoute
			}

			labels := []string{r.Method, route, strconv.Itoa(rec.status)}
			m.requests.WithLabelValues(labels...).Inc()
			m.durations.WithLabelValues(labels...).Observe(time.Since(start).Seconds())
		}()

		next.ServeHTTP(rec, r)
	})
}
