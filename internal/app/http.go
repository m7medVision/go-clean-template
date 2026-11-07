package app

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/controller/graphql"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// newHTTPHandler is the app's one HTTP mux. Every request is logged, counted, traced under /v1 and
// recovered from a panic. It serves /healthz, /metrics when metrics are on, GraphQL at /graphql (and
// GraphiQL when docs are on), and mounts the REST API, with its own token check and docs, on the
// rest.
func newHTTPHandler(cfg *config.Config, b cqrs.Buses, tokens *jwt.Manager, l *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.StripSlashes)
	r.Use(middleware.Logger(l))

	if cfg.Tracing.Enabled {
		r.Use(middleware.Tracing())
	}

	if cfg.Metrics.Enabled {
		r.Use(middleware.NewMetrics(prometheus.DefaultRegisterer).Middleware)
	}

	r.Use(middleware.Recovery(l))

	if cfg.Metrics.Enabled {
		r.Get("/metrics", promhttp.Handler().ServeHTTP)
	}

	r.Get("/healthz", healthz)

	r.Handle("/graphql", graphql.NewHandler(b, tokens, l))

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
