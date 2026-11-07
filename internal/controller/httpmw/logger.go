// Package httpmw is the middleware the app's HTTP mux wraps every request in, whatever serves it
// (REST, GraphQL, the docs, /healthz, /metrics): the request log, tracing, metrics and panic
// recovery. REST's own Huma middleware, the token check, is in restapi/middleware.
package httpmw

import (
	"log/slog"
	"net"
	"net/http"
)

type statusRecorder struct {
	http.ResponseWriter

	status      int
	bytes       int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status = status
		r.wroteHeader = true
	}

	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	r.wroteHeader = true

	written, err := r.ResponseWriter.Write(body)
	r.bytes += written

	return written, err
}

// Unwrap lets http.ResponseController reach the underlying writer for
// Flush, Hijack and the read and write deadlines.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func Logger(l *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			defer func() {
				l.Info("http request", "client", clientAddr(r), "method", r.Method, "uri", r.URL.RequestURI(),
					"status", rec.status, "bytes", rec.bytes)
			}()

			next.ServeHTTP(rec, r)
		})
	}
}

func clientAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
