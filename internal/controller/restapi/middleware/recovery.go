package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/evrone/go-clean-template/pkg/logger"
)

// Recovery returns a middleware that recovers from panics, logs the panic
// message with a stack trace, and answers with 500 Internal Server Error.
func Recovery(l logger.Interface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					l.Error(buildPanicMessage(r, rec))

					w.WriteHeader(http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
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
