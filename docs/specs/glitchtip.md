# Spec: backend error tracking in GlitchTip

Module id: `glitchtip` · see [README.md](README.md#observability-and-messaging) · Depends on nothing
(built after `nats-queue` so its given-up messages are covered) · Shares the GlitchTip server and
release name with [web-error-tracking.md](web-error-tracking.md) (done, #53) · Status: **approved
2026-11-10, built on `docs/glitchtip-spec`, awaiting review**

## Objective

An unexpected error in the app or the worker becomes a GlitchTip issue, with the log lines that led
to it, so the team is told something broke instead of finding it in stdout. Done means:

- a 500 on any transport, a recovered panic, a message given up by the worker and a projection stuck
  on an event each appear once in GlitchTip, grouped by what failed, with the call site's stack;
- each event carries the earlier log lines of its own request or message as breadcrumbs, never
  another request's;
- an expected failure (not found, forbidden, invalid, exists, rate limited) is never sent;
- no client address, URI, body, header or email leaves the process;
- with no `SENTRY_DSN`, nothing is sent and nothing changes.

**Who it is for:** the team running a service built from the template, who need an alert, not a log.

## Design (decided)

1. **One adapter, no strategy interface.** GlitchTip, Sentry and every other server that speaks the
   Sentry protocol are chosen by `SENTRY_DSN` alone, through the Sentry Go SDK
   (`github.com/getsentry/sentry-go`). The seam a vendor plugs into is `slog.Handler`: a vendor
   without the Sentry protocol would be one more handler wrapper beside `pkg/sentrylog`, and two at
   once a small fan-out handler. Neither is built until one is needed. (Analytics needed its own
   `AnalyticsSink` because each vendor is delivered through the queue with its own retries;
   an error report is sent in-process, and the SDK buffers and retries.)
2. **The `slog.Handler` wrapper is the only thing that calls the SDK to report.** No port, no
   handler or transport change. An `Error` record becomes an event, its `error` attribute the
   exception; a record below `Error` becomes a breadcrumb. The transports' error mappers log only
   `KindInternal` and `KindUpstream` at `Error` (`entity.Kind.LogLevel`), so expected failures are
   never sent.
3. **One hub per unit of work.** Each HTTP request, gRPC call, queue message and projection
   subscriber loop gets a clone of the root hub on its context (`sentrylog.Fork`). A record whose
   context carries no hub is never a breadcrumb (it would pile up on the global hub across
   requests); an `Error` record with no hub is still sent, with no breadcrumbs. The panic recoveries
   move from `l.Error` to `l.ErrorContext`, so a panic gets its request's breadcrumbs.
4. **Breadcrumbs need the debug lines.** With a DSN the wrapper's `Enabled` is always true and only
   the inner handler applies `LOG_LEVEL`, so `cqrs - message failed` (a `Debug` line) reaches the
   breadcrumbs while stdout stays at `info`. That builds every debug record when the DSN is set: a
   small CPU cost, the price of useful breadcrumbs. With no DSN the logger is not wrapped.
5. **Grouping:** every 500 is logged from the same mapper line, and the repo's `fmt.Errorf` chains
   carry no stack, so a stack-based group would merge them. The event's fingerprint is
   `{record message, outermost operation}`, the operation being the error text before its first
   `": "` (`CreateTask - aggregate.Save`). The stack is the logging call site (`AttachStacktrace`).
6. **Attributes on an allowlist.** Events and breadcrumbs carry only `op`, `name`, `kind`,
   `duration`, `subscriber`, `topic`, `status`, `method` (an HTTP verb or gRPC method name) and
   `panic` (a recovered panic's value, which also stands in for the operation in its fingerprint).
   Anything else, `client` and `uri` included, is dropped; a new attribute is added on purpose.
7. **Tags:** `service` (`app` or `worker`), `trace_id` (from the OTel span in `ctx`), `release`
   (`APP_VERSION`, required, the same string the web build sends), `environment`. User: the ID only,
   set by `internal/controller/authn` once the caller is known (`sentrylog.SetUser`).
8. **A stuck projection is reported once per position.** The runner logs `Error` the first time a
   subscriber fails at an event position and `Warn` while it keeps failing there; a new position, or
   a success, starts again. `ensureCheckpoint` logs `Warn` for the first minute (`_checkpointGrace`:
   the worker may start before the app has applied the schema), then `Error` once.
9. **One GlitchTip project each** for the app, the worker and the web. Both binaries read the same
   `SENTRY_DSN`; compose gives each service its own value.
10. **GlitchTip's logs feature and performance tracing are not used.** Logs stay on stdout, traces in
    the OTel backend.
11. **On by `SENTRY_DSN`'s presence**, in both binaries; no toggle. Flushed on shutdown, after the
    servers stop. An error that stops a binary before bootstrap (`cmd/*/main.go`'s own logger) is not
    sent: it is on stdout and the process exits.

## Configuration

| Env var | Default | Read by |
| --- | --- | --- |
| `SENTRY_DSN` | empty: off | the app and the worker, each its own project's DSN |
| `SENTRY_ENVIRONMENT` | `development` | the app and the worker |

```yaml
# docker-compose.yml
app:    { environment: { SENTRY_DSN: "${SENTRY_APP_DSN:-}" } }
worker: { environment: { SENTRY_DSN: "${SENTRY_WORKER_DSN:-}" } }
```

## Project structure

| Where | Change |
| --- | --- |
| `config/sentry.go` | the `Sentry` section; listed in `config/config.go` |
| `pkg/sentrylog/sentrylog.go` | `Options`, `Init` (returns the flush), the `Handler` wrapper |
| `pkg/sentrylog/event.go` | `event`, `breadcrumb`, `operation` (the fingerprint), `allowed` (the allowlist) |
| `pkg/sentrylog/hub.go` | `Fork`, `HTTP`, the gRPC unary and stream interceptors, `Watermill`, `SetUser` |
| `internal/app/bootstrap.go` | `Init` and the wrapped logger when the DSN is set; the flush among the stops |
| `internal/app/http.go`, `grpc.go`, `worker.go` | the hub middlewares, right after tracing |
| `pkg/broker/broker.go` | `sentrylog.Watermill` first among the router's middlewares, so a message's retries share its hub (`broker.Settings` stays the shape of `config.Worker`) |
| `internal/controller/authn` | `sentrylog.SetUser(ctx, id)` |
| `internal/controller/httpmw/recovery.go`, `grpc/middleware/recovery.go` | `Error` → `ErrorContext` |
| `internal/repo/persistent/projection/runner.go` | `Fork` per subscriber loop, `eventError{position}`, `report`, the checkpoint grace |
| `.env.example`, `docker-compose.yml` | the two settings, a DSN per service |
| `docs/configuration.md`, `docs/architecture.md` (`pkg/sentrylog`), a new `docs/error-tracking.md`, `docs/worker.md` | what is sent and what is not; the runner's reporting |

## Code shape

The SDK calls are checked against the current `sentry-go` docs before building.

`pkg/sentrylog/sentrylog.go`:

```go
type Options struct {
	DSN, Environment, Release, Service string
}

func Init(o Options) (func(), error) {
	err := sentry.Init(sentry.ClientOptions{
		Dsn: o.DSN, Environment: o.Environment, Release: o.Release,
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

// root takes the Error records whose context carries no hub; bootstrap passes sentry.CurrentHub().
func New(next slog.Handler, root *sentry.Hub) *Handler { return &Handler{next: next, root: root} }

func (h *Handler) Enabled(context.Context, slog.Level) bool { return true }

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
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

	return h.next.Handle(ctx, r)
}
```

`WithAttrs` and `WithGroup` copy the handler and pass the call through to `next`.

`pkg/sentrylog/event.go`:

```go
func event(ctx context.Context, r slog.Record, base []slog.Attr) *sentry.Event {
	ev := sentry.NewEvent()
	ev.Level = sentry.LevelError
	ev.Message = r.Message

	var err error

	each(r, base, func(a slog.Attr) {
		if e, ok := a.Value.Any().(error); ok && a.Key == "error" {
			err = e
		} else if allowed(a.Key) {
			ev.Extra[a.Key] = a.Value.String()
		}
	})

	if err != nil {
		ev.SetException(err, _maxErrorDepth)
	}

	ev.Fingerprint = []string{r.Message, operation(err)}

	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		ev.Tags = map[string]string{"trace_id": sc.TraceID().String()}
	}

	return ev
}

func operation(err error) string {
	if err == nil {
		return ""
	}

	op, _, _ := strings.Cut(err.Error(), ": ")

	return op
}

func allowed(key string) bool {
	switch key {
	case "op", "name", "kind", "duration", "subscriber", "topic", "status", "method":
		return true
	}

	return false
}
```

`breadcrumb` is the same idea: `Category` is the message before its first `" - "`, the level mapped,
`Data` only the allowed attributes.

`pkg/sentrylog/hub.go`:

```go
func Fork(ctx context.Context) context.Context {
	parent := sentry.GetHubFromContext(ctx)
	if parent == nil {
		parent = sentry.CurrentHub()
	}

	return sentry.SetHubOnContext(ctx, parent.Clone())
}

func HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(Fork(r.Context())))
	})
}

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		return h(Fork(ctx), req)
	}
}

func Watermill(h message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		msg.SetContext(Fork(msg.Context()))

		return h(msg)
	}
}

func SetUser(ctx context.Context, id string) {
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		hub.Scope().SetUser(sentry.User{ID: id})
	}
}
```

The stream interceptor wraps the stream's context the same way. With no DSN, `Init` is never called
and every cloned hub has no client, so it sends nothing; the middlewares stay installed, so the
wiring does not branch on the DSN.

`internal/app/bootstrap.go`:

```go
l := newLogger(cfg.Log.Level)

if cfg.Sentry.DSN != "" {
	flush, err := sentrylog.Init(sentrylog.Options{
		DSN: cfg.Sentry.DSN, Environment: cfg.Sentry.Environment,
		Release: cfg.App.Version, Service: service,
	})
	if err != nil {
		return nil, fmt.Errorf("app - bootstrap - sentrylog.Init: %w", err)
	}

	b.stops = append(b.stops, flush)
	l = slog.New(sentrylog.New(l.Handler(), sentry.CurrentHub()))
}
```

`internal/repo/persistent/projection/runner.go`:

```go
func (r *Runner) run(ctx context.Context, s repo.Subscriber, wake <-chan struct{}) {
	ctx = sentrylog.Fork(ctx)
	var failed int64

	for ctx.Err() == nil {
		full, err := r.batch(ctx, s, handles)

		switch {
		case full, errors.Is(err, errMoved):
			continue
		case err != nil && !errors.Is(err, errBusy):
			failed = r.report(ctx, s, err, failed)
		default:
			failed = 0
		}
		// wait as today
	}
}

func (r *Runner) report(ctx context.Context, s repo.Subscriber, err error, failed int64) int64 {
	var at *eventError
	pos := int64(-1)

	if errors.As(err, &at) {
		pos = at.position
	}

	level := slog.LevelError
	if pos == failed {
		level = slog.LevelWarn
	}

	r.logger.Log(ctx, level, "projection - handle an event", "subscriber", s.Name(), "error", err)

	return pos
}
```

`batch` wraps a subscriber's failure in `eventError{position, err}`.

## Testing strategy

Unit, in `pkg/sentrylog` with a fake `sentry.Transport`:

| Case | Expectation |
| --- | --- |
| An `Error` record | one event: `trace_id`, `service`, the fingerprint, the breadcrumbs before it |
| `Debug`, `Info`, `Warn` records | breadcrumbs only, never sent on their own |
| `Debug` with `LOG_LEVEL=info` | a breadcrumb, and nothing on stdout |
| `client`, `uri`, `email` attributes | absent from the event and the breadcrumbs |
| Two concurrent forked contexts | their breadcrumbs do not mix |
| No hub in the context, `Info` | no breadcrumb on the global hub |
| Two errors of different operations through one call site | two fingerprints |

Elsewhere:

- The mappers (`apierr`, `rpcerr`, `gqlerr`): a 404, 403, 409 (exists), 422 and 429 log nothing at
  `Error` (already true; the test pins it).
- The runner: the same position failing three times is one `Error` and two `Warn`; a checkpoint
  failing within the grace is only `Warn`, then one `Error`.
- `internal/app`: no DSN, the logger is not wrapped and no outbound call is made.
- **By hand:** a forced 500, a panic, a given-up worker message and a stuck projection each show up
  once in a GlitchTip, with breadcrumbs and `trace_id`.

## Boundaries

- **Always:** attributes on the allowlist only; the user ID and nothing else about the user; the
  repo's checks.
- **Ask first:** GlitchTip in compose (decided 2026-11-08: not at the start; the DSN points at a
  GlitchTip you run); performance tracing to GlitchTip; sending below `Error`; a second vendor.
- **Never:** a handler or use case calling the SDK; Sentry SaaS-only features; a strategy interface
  before a non-Sentry vendor exists.

## Success criteria

1. A forced 500, a panic, a given-up worker message and a stuck projection each produce one
   GlitchTip event with breadcrumbs of their own request or message, `trace_id` and `service`.
2. Two different 500s are two issues.
3. A 404, 403, 409, 422 or 429 produces none.
4. No client address, URI or email in any event or breadcrumb.
5. No `SENTRY_DSN`: no outbound call, behaviour unchanged.
6. The app, worker and web events land in their own projects under the same release name.
