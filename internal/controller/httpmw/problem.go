package httpmw

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

const _problemTypeBlank = "about:blank"

type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

// WriteProblem writes an RFC 9457 problem response, as Huma writes its own, for the answers a
// middleware gives before or instead of a handler: a panic, or REST's missing token.
func WriteProblem(l *slog.Logger, w http.ResponseWriter, status int, detail string) {
	body, err := json.Marshal(problem{
		Type:   _problemTypeBlank,
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	})
	if err != nil {
		l.Error("httpmw - write a problem", "error", err)

		return
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	if _, err := w.Write(body); err != nil {
		l.Error("httpmw - write a problem", "error", err)
	}
}
