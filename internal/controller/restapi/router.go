// Package restapi contains server handlers for the HTTP transport, built on chi
// and Huma. The docs UI (Stoplight Elements) and the OpenAPI spec are served by
// Huma itself at /docs and /openapi.[json|yaml].
package restapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/middleware"
	"github.com/m7medvision/go-clean-template/internal/controller/restapi/v1/apidoc"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

const _apiTitle = "Go Clean Template API"

// NewHandler serves the REST API: the operations register adds to it, and, when docs is on, the
// docs UI, the OpenAPI document stating version, and its schemas. An operation that declares
// Security needs a token jwtManager signed; one that declares none, and the docs, are public.
func NewHandler(
	version string, docs bool, jwtManager *jwt.Manager, l *slog.Logger, register func(api huma.API),
) http.Handler {
	r := chi.NewRouter()

	api := newAPI(r, version, docs)
	// Huma wraps an operation in the middleware registered before it, so this goes first.
	api.UseMiddleware(middleware.Auth(jwtManager, l))
	register(api)

	return r
}

// Document returns the OpenAPI document of the operations register adds, stating version, as
// NewHandler serves it. Registering an operation calls no handler, so register may pass empty
// buses.
func Document(version string, register func(api huma.API)) ([]byte, error) {
	api := newAPI(chi.NewRouter(), version, true)
	register(api)

	b, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("restapi - Document - json.MarshalIndent: %w", err)
	}

	return b, nil
}

func newAPI(r chi.Router, version string, docs bool) huma.API {
	apiConfig := huma.DefaultConfig(_apiTitle, version)
	// The default hook adds a `$schema` field and a `Link` header to every response body.
	apiConfig.CreateHooks = nil
	apiConfig.Transformers = append(apiConfig.Transformers, hideErrorValues)

	if !docs {
		apiConfig.DocsPath = ""
		apiConfig.OpenAPIPath = ""
		apiConfig.SchemasPath = ""
	}

	api := humachi.New(r, apiConfig)
	api.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		apidoc.BearerAuthScheme: {
			Type: "apiKey",
			In:   "header",
			Name: "Authorization",
		},
	}

	return api
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
