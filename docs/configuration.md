# Configuration

Every setting is an environment variable, parsed by [`config/config.go`](../config/config.go) with
[caarlos0/env](https://github.com/caarlos0/env). There are no config files. Start from
[`.env.example`](../.env.example): the `Makefile` loads `.env` (falling back to `.env.example`) and
exports it into every target, and [docker-compose.yml](../docker-compose.yml) passes the same
variables to the containers.

Keep `.env` values free of spaces and quotes: both `make` and the shell read that file.

## App

The Default column is the value in `config.go`; "required" means the binary refuses to start without
it.

| Env var | Default | |
| --- | --- | --- |
| `APP_NAME` | required | |
| `APP_VERSION` | required | |
| `APP_BASE_URL` | `http://localhost` | where the web client is served; email links point under it |
| `HTTP_PORT` | required | REST, GraphQL, docs and metrics |
| `GRPC_PORT` | required | |
| `LOG_LEVEL` | required | `debug`, `info`, `warn`, `error` |
| `PG_URL` | required | the app's database. The startup schema apply makes it match the HCL schema, dropping what the schema doesn't declare |
| `PG_POOL_MAX` | required | |
| `JWT_SECRET` | required | change it outside development |
| `JWT_TOKEN_EXPIRY` | `24h` | |
| `METRICS_ENABLED` | `true` | Prometheus at `/metrics` |
| `DOCS_ENABLED` | `false` | `/docs`, `/openapi.*`, `/schemas/*` and the GraphQL playground |

Two variables are read by tooling, not by the app:

- `PG_PORT` (default `5432`) is the host port the compose `db` service publishes, and `PG_URL` in
  `.env.example` is built from it. Set it when something else on the host already uses 5432.
- `ATLAS_DEV_URL` is read only by `make sqlc`, which renders the HCL schema in a throwaway
  `docker://` Postgres container.

## Exchange

| Env var | Default | |
| --- | --- | --- |
| `EXCHANGE_SERVICE_URL` | `https://open.er-api.com` | base URL of the rate provider; the integration tests point it at a stub |

## Worker and email

Read only by `cmd/worker`, so none of them is required. See [worker.md](worker.md) for how they behave.

| Env var | Default | |
| --- | --- | --- |
| `WORKER_POLL_INTERVAL` | `1s` | sleep after an empty read |
| `WORKER_VISIBILITY_TIMEOUT` | `30s` | how long a read message is hidden before redelivery |
| `WORKER_BATCH_SIZE` | `10` | messages per read |
| `WORKER_SHUTDOWN_TIMEOUT` | `10s` | grace period for in-flight handlers |
| `SMTP_HOST` | `localhost` | SMTP relay (`mailpit` in compose) |
| `SMTP_PORT` | `1025` | |
| `SMTP_USERNAME` | empty | empty turns auth off; when set, PLAIN auth, sent only over TLS or to localhost |
| `SMTP_PASSWORD` | empty | |
| `MAIL_FROM` | `Go Clean Template <noreply@example.com>` | sender address; `.env.example` uses a bare address because of the no-spaces rule |

## Tracing

Distributed tracing uses [OpenTelemetry](https://opentelemetry.io/). Spans are exported over OTLP/gRPC
to a collector: [Jaeger](https://www.jaegertracing.io/) in the docker stack, with its UI at
http://127.0.0.1:16686.

- **One trace across transports.** W3C `traceparent` and `baggage` propagate through every transport.
  REST and GraphQL use the
  [otelhttp](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp)
  middleware, and gRPC uses the
  [otelgrpc](https://github.com/open-telemetry/opentelemetry-go-contrib) stats handler. The outbound
  exchange-rate call uses an `otelhttp` transport, so it joins the same trace.
- **Every layer has spans.** Use cases and repositories are wrapped in tracing decorators
  (`*/tracing.go`), so a trace shows controller → use case → repo or webapi.
- **No-op when disabled.** With `TRACING_ENABLED=false`, a no-op provider is installed, so the
  instrumentation runs unconditionally with no overhead.
- **Only the API is traced.** `/healthz`, `/metrics` and `/docs` are excluded; only the versioned
  API groups are instrumented.

| Env var | Default | |
| --- | --- | --- |
| `TRACING_ENABLED` | `false` | `.env.example` turns it on |
| `TRACING_OTLP_ENDPOINT` | `localhost:4317` | OTLP/gRPC collector |
| `TRACING_OTLP_INSECURE` | `true` | no TLS to the collector |
| `TRACING_SAMPLE_RATE` | `0.1` | parent-based ratio. Use `1.0` locally (`.env.example` does), lower it in production |
