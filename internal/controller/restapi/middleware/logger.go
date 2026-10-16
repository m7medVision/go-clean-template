package middleware

import (
	"fmt"
	"net"
	"net/http"

	"github.com/evrone/go-clean-template/pkg/logger"
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

// Logger returns a middleware that logs method, path, status code, and body size
// of every response.
func Logger(l logger.Interface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			defer func() {
				l.Info("%s", buildRequestMessage(r, rec))
			}()

			next.ServeHTTP(rec, r)
		})
	}
}

func buildRequestMessage(r *http.Request, rec *statusRecorder) string {
	return fmt.Sprintf("%s - %s %s - %d %d",
		clientAddr(r),
		r.Method,
		r.URL.RequestURI(),
		rec.status,
		rec.bytes,
	)
}

func clientAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
