package app

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql"
	"github.com/m7medvision/go-clean-template/internal/controller/httpmw"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	_healthzPath = "/healthz"
	_metricsPath = "/metrics"
)

// newHTTPHandler serves REST and GraphQL side by side. Every request is logged, traced (the global
// provider is a no-op when tracing is off), counted on metrics and recovered from a panic;
// METRICS_ENABLED decides only whether /metrics serves the registry.
func newHTTPHandler(cfg *config.Config, b cqrs.Buses, tokens *jwt.Manager, metrics *prometheus.Registry, l *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.StripSlashes)
	r.Use(httpmw.Logger(l))
	r.Use(httpmw.Tracing(_healthzPath, _metricsPath))
	r.Use(httpmw.NewMetrics(metrics).Middleware)
	r.Use(httpmw.Recovery(l))

	if cfg.Metrics.Enabled {
		r.Get(_metricsPath, promhttp.InstrumentMetricHandler(metrics, promhttp.HandlerFor(metrics, promhttp.HandlerOpts{})).ServeHTTP)
	}

	r.Get(_healthzPath, healthz)

	r.Handle("/graphql", graphql.NewHandler(b, tokens, cfg.Docs.Enabled, l))

	if cfg.Docs.Enabled {
		r.Handle("/graphql/playground", graphql.NewPlayground("/graphql"))
	}

	r.Mount("/", restapi.NewHandler(cfg.App.Version, cfg.Docs.Enabled, tokens, l, registerREST(b, l)))

	return r
}

// OpenAPI returns the REST OpenAPI document stating version, from the same registration the app
// serves, without a server, a database or any setting.
func OpenAPI(version string) ([]byte, error) {
	return restapi.Document(version, registerREST(cqrs.Buses{}, slog.New(slog.DiscardHandler)))
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
