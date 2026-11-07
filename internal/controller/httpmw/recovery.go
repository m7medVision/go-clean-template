package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery returns a middleware that recovers from panics, logs the panic
// message with a stack trace, and answers with 500 Internal Server Error.
// A panic after the response has started aborts the connection instead, so
// the client never mistakes a truncated body for a complete one.
func Recovery(l *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			defer func() {
				p := recover()
				if p == nil {
					return
				}

				if err, ok := p.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(p)
				}

				l.Error("http panic", "client", clientAddr(r), "method", r.Method, "uri", r.URL.RequestURI(),
					"panic", fmt.Sprint(p), "stack", string(debug.Stack()))

				if rec.wroteHeader {
					panic(http.ErrAbortHandler)
				}

				writeProblem(l, rec, http.StatusInternalServerError, "internal server error")
			}()

			next.ServeHTTP(rec, r)
		})
	}
}
