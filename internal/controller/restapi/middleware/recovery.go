package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/m7medvision/go-clean-template/pkg/logger"
)

// Recovery returns a middleware that recovers from panics, logs the panic
// message with a stack trace, and answers with 500 Internal Server Error.
// A panic after the response has started aborts the connection instead, so
// the client never mistakes a truncated body for a complete one.
func Recovery(l logger.Interface) func(http.Handler) http.Handler {
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

				l.Error(buildPanicMessage(r, p))

				if rec.wroteHeader {
					panic(http.ErrAbortHandler)
				}

				writeProblem(l, rec, http.StatusInternalServerError, "internal server error")
			}()

			next.ServeHTTP(rec, r)
		})
	}
}

func buildPanicMessage(r *http.Request, rec any) string {
	var result strings.Builder

	result.WriteString(clientAddr(r))
	result.WriteString(" - ")
	result.WriteString(r.Method)
	result.WriteString(" ")
	result.WriteString(r.URL.RequestURI())
	result.WriteString(" PANIC DETECTED: ")

	fmt.Fprintf(&result, "%v\n%s\n", rec, debug.Stack())

	return result.String()
}
