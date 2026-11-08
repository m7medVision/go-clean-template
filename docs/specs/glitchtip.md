# Spec: backend error tracking in GlitchTip

Module id: `glitchtip` · second of four (see [README.md](README.md#observability-and-messaging)) ·
Depends on nothing (built after `nats-queue` so its poison errors are covered) · Unblocks
[web-error-tracking.md](web-error-tracking.md) · Status: **spec written 2026-11-08, awaiting review**

## Objective

An unexpected error in the app or the worker becomes a GlitchTip issue, with the log lines that led
to it, so the team is told something broke instead of finding it in stdout. Done means:

- a 500 on any transport, a recovered panic and a message given up by the worker each appear once
  in GlitchTip, grouped, with a stack;
- each event carries the last log lines of its request or message as breadcrumbs;
- an expected failure (not found, forbidden, invalid, rate limited) is never sent;
- with no `SENTRY_DSN`, nothing is sent and nothing changes.

**Who it is for:** the team running a service built from the template, who need an alert, not a log.

## Design (decided)

1. **GlitchTip only**, self-hosted, through the Sentry Go SDK (`github.com/getsentry/sentry-go`).
2. **One `slog.Handler` wrapper**, no port, no handler or transport change. Records below `Error`
   become breadcrumbs on the request's or message's hub; an `Error` record becomes an event with its
   `error` attribute as the exception. The error mappers, recovery and the worker already log every
   unexpected error at `Error`, so they are covered.
3. **Tags:** `trace_id` (from the OTel span in `ctx`), the user ID only, `release` (the build's
   version, the same string the web client sends), `environment`, the message name.
4. **GlitchTip's logs feature is not used.** Logs stay on stdout.
5. **On by `SENTRY_DSN`'s presence**, in both binaries; no toggle.

```go
func (h Handler) Handle(ctx context.Context, r slog.Record) error {
	hub := sentry.GetHubFromContext(ctx)
	if r.Level >= slog.LevelError {
		hub.CaptureEvent(event(ctx, r))
	} else {
		hub.AddBreadcrumb(breadcrumb(r), nil)
	}

	return h.next.Handle(ctx, r)
}
```

## Configuration

| Env var | Default | Read by |
| --- | --- | --- |
| `SENTRY_DSN` | empty: off | the app and the worker |
| `SENTRY_ENVIRONMENT` | `development` | the app and the worker |

## Project structure

| Where | Change |
| --- | --- |
| `pkg/sentrylog/` | the handler wrapper, breadcrumbs, event building, a per-request/per-message hub |
| `internal/app/bootstrap.go` | the logger wrapped when the DSN is set; flushed on shutdown |
| `internal/controller/httpmw`, the gRPC interceptors, the worker router | a hub per request or message (clone of the root hub) |
| `config/config.go`, `.env.example`, `docker-compose.yml` | the two settings |
| `docs/configuration.md`, `docs/architecture.md`, a `docs/observability.md` section | what is sent, what is not |

## Testing strategy

- **Unit:** the wrapper with a fake transport: an `Error` record is one event with the right tags
  and breadcrumbs; below `Error` sends nothing; an expected entity error logged by a mapper sends
  nothing; no DSN, no transport calls. Breadcrumbs of two concurrent requests do not mix.
- **By hand:** a forced 500 shows in a GlitchTip with its trace ID.

## Boundaries

- **Always:** no PII in events (user ID only, no bodies, no headers); the repo's checks.
- **Ask first:** GlitchTip in compose (decided 2026-11-08: not at the start; the DSN points at a GlitchTip you run); performance tracing to GlitchTip; capturing below `Error`.
- **Never:** a handler calling the SDK; Sentry SaaS-only features.

## Success criteria

1. A forced 500, a panic and a given-up worker message each produce one GlitchTip event with
   breadcrumbs and `trace_id`.
2. A 404, 403, 422 or 429 produces none.
3. No `SENTRY_DSN`: no outbound call, behaviour unchanged.
4. `web-error-tracking` can ship: the same GlitchTip and release name.

## Open questions

1. One GlitchTip project for app, worker and web, or one each?
