package sentrylog

import (
	"context"
	"log/slog"
	"strings"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/otel/trace"
)

const _maxErrorDepth = 10

func event(ctx context.Context, r *slog.Record, base []slog.Attr) *sentry.Event {
	ev := sentry.NewEvent()
	ev.Level = sentry.LevelError
	ev.Message = r.Message
	attrs := sentry.Context{}

	var (
		err      error
		panicked string
	)

	each(r, base, func(a slog.Attr) {
		if a.Key == "panic" {
			panicked = a.Value.String()
		}

		if e, ok := a.Value.Any().(error); ok && a.Key == "error" {
			err = e
		} else if allowed(a.Key) {
			attrs[a.Key] = a.Value.String()
		}
	})

	ev.Contexts["log"] = attrs

	if err != nil {
		ev.SetException(err, _maxErrorDepth)
	}

	ev.Fingerprint = []string{r.Message, operation(err) + panicked}

	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		ev.Tags["trace_id"] = sc.TraceID().String()
	}

	return ev
}

func breadcrumb(r *slog.Record, base []slog.Attr) *sentry.Breadcrumb {
	category, _, _ := strings.Cut(r.Message, " - ")
	data := map[string]any{}

	each(r, base, func(a slog.Attr) {
		if allowed(a.Key) {
			data[a.Key] = a.Value.String()
		}
	})

	return &sentry.Breadcrumb{
		Category:  category,
		Message:   r.Message,
		Data:      data,
		Level:     level(r.Level),
		Timestamp: r.Time,
	}
}

func each(r *slog.Record, base []slog.Attr, fn func(slog.Attr)) {
	for _, a := range base {
		fn(a)
	}

	r.Attrs(func(a slog.Attr) bool {
		fn(a)

		return true
	})
}

// operation groups "CreateTask - aggregate.Save: conn refused" as "CreateTask - aggregate.Save".
func operation(err error) string {
	if err == nil {
		return ""
	}

	op, _, _ := strings.Cut(err.Error(), ": ")

	return op
}

func allowed(key string) bool {
	switch key {
	case "op", "name", "kind", "duration", "subscriber", "topic", "status", "method", "panic":
		return true
	}

	return false
}

func level(l slog.Level) sentry.Level {
	switch {
	case l >= slog.LevelError:
		return sentry.LevelError
	case l >= slog.LevelWarn:
		return sentry.LevelWarning
	case l >= slog.LevelInfo:
		return sentry.LevelInfo
	}

	return sentry.LevelDebug
}
