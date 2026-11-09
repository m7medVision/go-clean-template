# Error tracking

An unexpected error in the app or the worker becomes an issue in GlitchTip, with the log lines that
led to it. Any server that speaks the Sentry protocol works the same way (GlitchTip, Sentry): the
DSN picks it. With no `SENTRY_DSN` nothing is sent and nothing changes. The web client reports its
own errors to its own project ([web.md](web.md)).

## How it works

`pkg/sentrylog` wraps the binary's `slog` handler; nothing else calls the Sentry SDK.

- **An `Error` record is an event**, its `error` attribute the exception. The transports' error
  mappers log only internal and upstream failures at `Error` (`entity.Kind.LogLevel`), so a 404, 403,
  409, 422 or 429 is never sent. A recovered panic, a message the worker gave up on and a stuck
  projection are logged at `Error` too.
- **A record below `Error` is a breadcrumb** on the hub of its request, gRPC call, queue message or
  projection loop (`sentrylog.Fork`, installed right after tracing), so an event shows what its own
  unit of work did and never another's. Breadcrumbs include `Debug` lines even when `LOG_LEVEL` keeps
  them off stdout; a line whose context carries no hub is never a breadcrumb.
- **Grouping:** an event's fingerprint is its log message and the outermost operation of its error
  (`CreateTask - aggregate.Save`), or the panic's value, so two different 500s are two issues even
  though one mapper line logs both. The stack is the logging call site.
- **What leaves the process:** only the attributes `op`, `name`, `kind`, `duration`, `subscriber`,
  `topic`, `status`, `method` and `panic`; never a client address, URI, body, header or email. The
  user is their ID only. Tags: `service` (`app` or `worker`), `trace_id`, `release` (`APP_VERSION`),
  `environment`.
- **A stuck projection is one issue.** The runner logs `Error` the first time a subscriber fails at
  an event and `Warn` while it keeps failing there. A checkpoint that cannot be created (the worker
  started before the app applied the schema) is a `Warn` for the first minute, then one `Error`.
- **Flushed on shutdown**, after the servers stop. An error that stops a binary before it has a
  logger is on stdout only.

GlitchTip's logs and performance tracing are not used: logs stay on stdout, traces in the OTel
backend. The template does not run a GlitchTip: point the DSN at one you run.

## Projects

Each binary reports to its own project: the app, the worker and the web client each have one.
Both binaries read `SENTRY_DSN`; compose gives each service its own value from `SENTRY_APP_DSN` and
`SENTRY_WORKER_DSN`. See [configuration.md](configuration.md#error-tracking).
