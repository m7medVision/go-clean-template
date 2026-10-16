package middleware

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const _tracerName = "rest-api"

// Tracing returns a middleware that opens an OTel span for the versioned API
// routes only, keeping the internal routes (health, metrics, docs) untraced.
func Tracing() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		traced := otelhttp.NewHandler(next, _tracerName)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, _apiPrefix) {
				next.ServeHTTP(w, r)

				return
			}

			traced.ServeHTTP(w, r)
		})
	}
}
