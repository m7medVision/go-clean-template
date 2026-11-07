// Package restapi contains server handlers for the HTTP transport, built on chi
// and Huma. The docs UI (Stoplight Elements) and the OpenAPI spec are served by
// Huma itself at /docs and /openapi.[json|yaml].
package restapi

import (
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	v1 "github.com/m7medvision/go-clean-template/internal/controller/restapi/v1"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const _apiTitle = "Go Clean Template API"

// Settings say what the router serves besides the operations.
type Settings struct {
	// Version is the API's version, as the OpenAPI document states it.
	Version string
	// Docs serves the docs UI, the OpenAPI document and its schemas.
	Docs bool
	// Tracing opens a span per API request; Metrics counts requests and serves /metrics.
	Tracing bool
	Metrics bool
}

// NewRouter builds the HTTP handler for every REST surface of the app.
func NewRouter(
	s Settings,
	b cqrs.Buses,
	jwtManager *jwt.Manager,
	l *slog.Logger,
	opts ...Option,
) *chi.Mux {
	apiConfig := newAPIConfig(s.Version)

	if !s.Docs {
		apiConfig.DocsPath = ""
		apiConfig.OpenAPIPath = ""
		apiConfig.SchemasPath = ""
	}

	r := chi.NewRouter()

	r.Use(chimw.StripSlashes)
	r.Use(middleware.Logger(l))

	if s.Tracing {
		r.Use(middleware.Tracing())
	}

	if s.Metrics {
		prometheusMiddleware := middleware.NewMetrics(prometheus.DefaultRegisterer)
		r.Use(prometheusMiddleware.Middleware)
	}

	r.Use(middleware.Recovery(l))
	r.Use(middleware.Auth(jwtManager, l))

	if s.Metrics {
		r.Get("/metrics", promhttp.Handler().ServeHTTP)
	}

	r.Get("/healthz", healthz)

	api := humachi.New(r, apiConfig)
	v1.NewRoutes(api, b, l)

	for _, opt := range opts {
		opt(api, b, l)
	}

	return r
}

// Option registers an optional feature's operations on the REST API, over the buses.
type Option func(api huma.API, b cqrs.Buses, l *slog.Logger)

// newAPIConfig is shared by NewRouter and OpenAPI so the exported document matches the served one.
func newAPIConfig(version string) huma.Config {
	apiConfig := huma.DefaultConfig(_apiTitle, version)
	// The default hook adds a `$schema` field and a `Link` header to every response body.
	apiConfig.CreateHooks = nil
	apiConfig.Transformers = append(apiConfig.Transformers, hideErrorValues)

	return apiConfig
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func hideErrorValues(_ huma.Context, _ string, v any) (any, error) {
	if model, ok := v.(*huma.ErrorModel); ok {
		for _, detail := range model.Errors {
			if detail != nil {
				detail.Value = nil
			}
		}
	}

	return v, nil
}
