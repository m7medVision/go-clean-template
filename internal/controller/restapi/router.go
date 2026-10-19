// Package restapi contains server handlers for the HTTP transport, built on chi
// and Huma. The docs UI (Stoplight Elements) and the OpenAPI spec are served by
// Huma itself at /docs and /openapi.[json|yaml].
package restapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/evrone/go-clean-template/config"
	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	v1 "github.com/evrone/go-clean-template/internal/controller/restapi/v1"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const _apiTitle = "Go Clean Template API"

// NewRouter builds the HTTP handler for every REST surface of the app.
func NewRouter(
	cfg *config.Config,
	e usecase.Exchange,
	u usecase.User,
	tk usecase.Task,
	jwtManager *jwt.Manager,
	l logger.Interface,
) *chi.Mux {
	apiConfig := newAPIConfig(cfg.App.Version)

	if !cfg.Docs.Enabled {
		apiConfig.DocsPath = ""
		apiConfig.OpenAPIPath = ""
		apiConfig.SchemasPath = ""
	}

	r := chi.NewRouter()

	r.Use(chimw.StripSlashes)
	r.Use(middleware.Logger(l))

	if cfg.Tracing.Enabled {
		r.Use(middleware.Tracing())
	}

	if cfg.Metrics.Enabled {
		prometheusMiddleware := middleware.NewMetrics(prometheus.DefaultRegisterer)
		r.Use(prometheusMiddleware.Middleware)
	}

	r.Use(middleware.Recovery(l))
	r.Use(middleware.Auth(jwtManager, l))

	if cfg.Metrics.Enabled {
		r.Get("/metrics", promhttp.Handler().ServeHTTP)
	}

	r.Get("/healthz", healthz)

	v1.NewRoutes(humachi.New(r, apiConfig), e, u, tk, l)

	return r
}

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
