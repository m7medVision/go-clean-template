// Package sentrylog reports unexpected errors to a Sentry-protocol server (GlitchTip, Sentry) from
// slog: an Error record becomes an event, a record below Error a breadcrumb on its context's hub.
package sentrylog

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
)

const _flushTimeout = 2 * time.Second

type Options struct {
	DSN         string
	Environment string
	Release     string
	Service     string
}

// Init installs the client on the root hub and returns the flush to run on shutdown.
func Init(o Options) (func(), error) {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              o.DSN,
		Environment:      o.Environment,
		Release:          o.Release,
		AttachStacktrace: true,
		Tags:             map[string]string{"service": o.Service},
	})
	if err != nil {
		return nil, fmt.Errorf("sentrylog - Init: %w", err)
	}

	return func() { sentry.Flush(_flushTimeout) }, nil
}

type Handler struct {
	next  slog.Handler
	root  *sentry.Hub
	attrs []slog.Attr
}

// New wraps next; root receives Error records whose context carries no hub.
func New(next slog.Handler, root *sentry.Hub) *Handler {
	return &Handler{next: next, root: root}
}

// Enabled is always true: breadcrumbs need the debug lines LOG_LEVEL keeps off stdout.
func (h *Handler) Enabled(context.Context, slog.Level) bool { return true }

func (h *Handler) Handle(ctx context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler fixes the signature
	hub := sentry.GetHubFromContext(ctx)

	switch {
	case r.Level >= slog.LevelError:
		if hub == nil {
			hub = h.root
		}

		hub.CaptureEvent(event(ctx, &r, h.attrs))
	case hub != nil:
		hub.AddBreadcrumb(breadcrumb(&r, h.attrs), nil)
	}

	if !h.next.Enabled(ctx, r.Level) {
		return nil
	}

	if err := h.next.Handle(ctx, r); err != nil {
		return fmt.Errorf("sentrylog - Handle: %w", err)
	}

	return nil
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		next:  h.next.WithAttrs(attrs),
		root:  h.root,
		attrs: append(h.attrs[:len(h.attrs):len(h.attrs)], attrs...),
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{next: h.next.WithGroup(name), root: h.root, attrs: h.attrs}
}
