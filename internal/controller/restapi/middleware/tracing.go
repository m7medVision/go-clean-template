package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	_tracerName = "rest-api"
	// _mountPattern is the pattern of a router mounted at the root, as chi reports it.
	_mountPattern = "/*"
)

// Tracing returns a middleware that opens an OTel span for every request but those to the untraced
// paths, such as the health check and the metrics scrape, which would only add noise. Every API is
// traced, whatever its version: REST and GraphQL alike.
func Tracing(untraced ...string) func(http.Handler) http.Handler {
	traced := func(r *http.Request) bool {
		return !slices.Contains(untraced, strings.TrimSuffix(r.URL.Path, "/"))
	}

	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(nameSpanByRoute(next), _tracerName, otelhttp.WithFilter(traced))
	}
}

// nameSpanByRoute renames the span after routing. otelhttp reads the pattern
// from its own *http.Request, and chi sets it on the copy it routes, so it
// cannot do this itself.
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

// routePattern is the pattern of the route that served r, or empty when none did. A request no
// route matched still carries the catch-all of the mount it went through.
func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return ""
	}

	if p := rctx.RoutePattern(); p != _mountPattern {
		return p
	}

	return ""
}
