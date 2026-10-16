package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "rest-api"

// Tracing returns a middleware that opens an OTel span for the versioned API
// routes only, keeping the internal routes (health, metrics, docs) untraced.
func Tracing() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		traced := otelhttp.NewHandler(nameSpanByRoute(next), _tracerName)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, _apiPrefix) {
				next.ServeHTTP(w, r)

				return
			}

			traced.ServeHTTP(w, r)
		})
	}
}

// nameSpanByRoute renames the span after routing. otelhttp reads the pattern
// from its own *http.Request, which chi never sees once Auth swaps in a
// request with the caller's context, so it cannot do this itself.
func nameSpanByRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			route := routePattern(r)
			if route == "" {
				return
			}

			span := trace.SpanFromContext(r.Context())
			span.SetName(r.Method + " " + route)
			span.SetAttributes(attribute.String("http.route", route))
		}()

		next.ServeHTTP(w, r)
	})
}

func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return ""
	}

	return rctx.RoutePattern()
}
