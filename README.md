![Go Clean Template](docs/img/logo.svg)

# Go Clean template

Clean Architecture template for Golang services

[![Release](https://img.shields.io/github/v/release/evrone/go-clean-template.svg)](https://github.com/evrone/go-clean-template/releases/)
[![License](https://img.shields.io/badge/License-MIT-success)](https://github.com/evrone/go-clean-template/blob/master/LICENSE)
[![codecov](https://codecov.io/gh/evrone/go-clean-template/branch/master/graph/badge.svg?token=XE3E0X3EVQ)](https://codecov.io/gh/evrone/go-clean-template)

[![Web Framework](https://img.shields.io/badge/Chi%20%2B%20Huma-Web%20Framework-blue)](https://github.com/danielgtaylor/huma)
[![API Documentation](https://img.shields.io/badge/OpenAPI-Documentation-blue)](https://github.com/danielgtaylor/huma)
[![JSON Handling](https://img.shields.io/badge/Go--JSON-Fast%20Serialization-blue)](https://github.com/goccy/go-json)
[![SQL](https://img.shields.io/badge/sqlc-Type--safe%20SQL-blue)](https://github.com/sqlc-dev/sqlc)
[![Database Migrations](https://img.shields.io/badge/Migrations-Seamless%20Schema%20Updates-blue)](https://atlasgo.io/)
[![Logging](https://img.shields.io/badge/ZeroLog-Structured%20Logging-blue)](https://github.com/rs/zerolog)
[![Metrics](https://img.shields.io/badge/Prometheus-Metrics%20Integration-blue)](https://github.com/prometheus/client_golang)
[![Tracing](https://img.shields.io/badge/OpenTelemetry-Distributed%20Tracing-blue)](https://opentelemetry.io/)
[![Testing](https://img.shields.io/badge/Testify-Testing%20Framework-blue)](https://github.com/stretchr/testify)
[![Mocking](https://img.shields.io/badge/Mock-Mocking%20Library-blue)](https://go.uber.org/mock)

## Overview

The purpose of the template is to show:

- how to organize a project and prevent it from turning into spaghetti code
- where to store business logic so that it remains independent, clean, and extensible
- how not to lose control when a microservice grows

Using the principles of Robert Martin (aka Uncle Bob).

[Go-clean-template](https://evrone.com/go-clean-template?utm_source=github&utm_campaign=go-clean-template) is created &
supported by [Evrone](https://evrone.com/?utm_source=github&utm_campaign=go-clean-template).

This template implements three types of servers:

- gRPC ([gRPC](https://grpc.io/) framework based on protobuf)
- REST API ([chi](https://github.com/go-chi/chi) router wrapped with the [Huma](https://github.com/danielgtaylor/huma) framework)
- GraphQL ([gqlgen](https://github.com/99designs/gqlgen), served at `/graphql` on the REST port: tasks and the exchange history)
- Reverse proxy ([Caddy](https://caddyserver.com/) with the caddy-waf firewall and the Souin HTTP cache)

Background work goes through a minimal queue in Postgres (one `queue_messages` table), consumed by
a separate worker binary (`cmd/worker`). Registration uses it to send a welcome email (Go `html/template`
templates embedded in the binary, delivered over SMTP), and an example queue shows the bare shape.

It also ships a React web client in `web/` ([TanStack Router](https://tanstack.com/router) +
[TanStack Query](https://tanstack.com/query), built with [Vite+](https://viteplus.dev/)) that calls the REST
API through types generated from the OpenAPI document.

The template includes three domains to demonstrate multi-service architecture:

- **User Authentication** — registration, login, JWT-based authorization
- **Task Management** — CRUD operations with status transitions (todo, in_progress, done)
- **Currency Exchange** — currency conversion at daily rates (about 160 currencies), with history and a web page

All domains are available over REST and gRPC. GraphQL exposes tasks and the exchange history.

## Content

- [Domains](#domains)
- [Quick start](#quick-start)
- [Observability](#observability)
- [Project structure](#project-structure)
- [Dependency Injection](#dependency-injection)
- [Clean Architecture](#clean-architecture)

## Domains

The template includes three fully implemented domains, each available over REST and gRPC. GraphQL covers tasks and the exchange history.

### User Authentication

Registration, login, and JWT-based authorization.

| Operation   | REST                     | gRPC                     |
|-------------|--------------------------|--------------------------|
| Register    | `POST /v1/auth/register` | `AuthService/Register`   |
| Login       | `POST /v1/auth/login`    | `AuthService/Login`      |
| Get profile | `GET /v1/user/profile`   | `AuthService/GetProfile` |

- Passwords hashed with bcrypt
- JWT tokens with configurable expiry
- Auth middleware on all transports
- A welcome email on registration, queued by the use case and sent by the worker
  (see [`internal/repo/mailtmpl`](#internalrepomailtmpl))

### Task Management

CRUD operations with a status state machine.

| Operation  | REST                         | gRPC                         | GraphQL (`POST /graphql`) |
|------------|------------------------------|------------------------------|---------------------------|
| Create     | `POST /v1/tasks`             | `TaskService/CreateTask`     | `mutation createTask`     |
| List       | `GET /v1/tasks`              | `TaskService/ListTasks`      | `query tasks`             |
| Get        | `GET /v1/tasks/:id`          | `TaskService/GetTask`        | `query task`              |
| Update     | `PUT /v1/tasks/:id`          | `TaskService/UpdateTask`     | `mutation updateTask`     |
| Transition | `PATCH /v1/tasks/:id/status` | `TaskService/TransitionTask` | `mutation transitionTask` |
| Delete     | `DELETE /v1/tasks/:id`       | `TaskService/DeleteTask`     | `mutation deleteTask`     |

- Status transitions: `todo` → `in_progress` → `done` (and `in_progress` → `todo`)
- Pagination with `limit`/`offset` and optional status filter
- Tasks scoped to the authenticated user

### Currency Exchange

Converts an amount between currencies at the latest daily rate, fetched from the
[ExchangeRate-API open access endpoint](https://www.exchangerate-api.com/docs/free) (keyless, about 160
currencies including the Gulf currencies, e.g. Omani Rial → Chinese Yuan), and keeps each
user's history. It is the template's example of an outbound HTTP API behind a repository
interface ([`internal/repo/webapi`](#internalrepowebapi)).

| Operation | REST                         | gRPC                         | GraphQL (`POST /graphql`) |
|-----------|------------------------------|------------------------------|---------------------------|
| Convert   | `POST /v1/exchange/convert`  | `ExchangeService/Convert`    |                           |
| History   | `GET /v1/exchange/history`   | `ExchangeService/GetHistory` | `query exchangeHistory`   |

- Request: `{"from": "OMR", "to": "CNY", "amount": "100.10"}`; codes are case-insensitive
- Amounts and rates are decimal strings, computed with exact decimals (no `float64`); the result is
  rounded half-even to 2 places
- Rates update once a day and are not live market rates; the response carries the rate's `rate_date`.
  The provider's terms require a link back to it wherever the rates are shown (the web page has one)
- Unknown currency or `from` = `to` → 400 (`InvalidArgument` on gRPC); history is paged with `limit`/`offset` and returns a `total`

## Quick start

### Local development

`make dev` runs everything in one [tmux](https://github.com/tmux/tmux) window. Postgres and Mailpit run in docker. The
app and the worker run on the host under [air](https://github.com/air-verse/air) and rebuild when you save a file. The
Vite+ web dev server runs on http://localhost:3000. Tracing is off and Jaeger is not started. It needs `tmux`, `curl`
and the Vite+ `vp` CLI (`make web-deps` once first).

```sh
make dev       # attach; run it again to re-attach after detaching (prefix + d)
make dev-down  # close the tmux session and stop Postgres and Mailpit
```

The four panes are:

- **app**: `.air.app.toml`, built with `-tags migrate`. It rebuilds on `.go`, `.hcl` and `.tmpl` changes, so saving
  a schema file re-syncs the database.
- **worker**: `.air.worker.toml`. It starts once the app answers `/healthz`, because only the app applies the schema.
- **web**: the Vite+ dev server.
- **db + mailpit**: the container logs.

If something else on the host already listens on 5432, set `PG_PORT` in `.env` (for example `PG_PORT=5433`). It moves
the published port of the compose `db` service and the port in `PG_URL` together. Without it, the app connects to
whatever already owns 5432, and the startup schema apply will try to make that database match this schema.

The same pieces without tmux, one terminal each:

```sh
# Postgres, and Mailpit to catch email (inbox on http://127.0.0.1:8025)
make compose-up
# Run app with migrations
make run
# Run the queue worker
make run-worker
```

### Web client (needs the [Vite+](https://viteplus.dev/) `vp` CLI)

```sh
make web-deps
# Vite+ dev server on http://localhost:3000, proxying the API to the app on HTTP_PORT
make web-dev
```

### Integration tests (can be run in CI)

```sh
# DB, app + migrations, worker, Mailpit, integration tests
make compose-up-integration-test
```

### Full docker stack with reverse proxy

```sh
make compose-up-all 
```

The reverse proxy is [Caddy](https://caddyserver.com/), built from [`caddy/Dockerfile`](caddy/Dockerfile) with two
modules and configured in [`caddy/Caddyfile`](caddy/Caddyfile). The same image builds `web/` with Vite+ and serves
it on `app.lvh.me`: the paths the Go app owns (`/v1`, `/graphql`, `/docs`, `/openapi.*`, `/schemas`, `/healthz`,
`/metrics`) go to the app, and everything else is the web client, with a Content-Security-Policy and history
fallback to `index.html`. The client and the API share one origin, so there is no CORS.

- [caddy-waf](https://github.com/fabriziosalmi/caddy-waf) inspects every request to `app.lvh.me` with its bundled
  rule set and answers `403` to a match. It is licensed AGPL-3.0, unlike this template.
- [Souin](https://github.com/darkweak/souin) caches the public documents (`/docs`, `/openapi.*`, `/schemas/*`) in
  memory and reports a `Cache-Status` header. API responses are per caller and are never cached.

Check services:

- Web client: http://app.lvh.me/ | http://localhost:3000 (`make web-dev`)
- REST API:
  - http://app.lvh.me/healthz | http://127.0.0.1:8080/healthz
  - http://app.lvh.me/metrics | http://127.0.0.1:8080/metrics
  - http://app.lvh.me/docs | http://127.0.0.1:8080/docs
  - http://app.lvh.me/openapi.json | http://127.0.0.1:8080/openapi.json
  - http://app.lvh.me/openapi.yaml | http://127.0.0.1:8080/openapi.yaml
  - http://app.lvh.me/schemas/{schema} | http://127.0.0.1:8080/schemas/{schema}
- GraphQL:
  - http://app.lvh.me/graphql | http://127.0.0.1:8080/graphql (POST only, `Authorization: Bearer <token>`)
  - Schemas: [task.graphqls](internal/controller/graphql/schema/task.graphqls),
    [exchange.graphqls](internal/controller/graphql/schema/exchange.graphqls)
  - Playground (when `DOCS_ENABLED=true`): http://app.lvh.me/graphql/playground | http://127.0.0.1:8080/graphql/playground.
    Set `{"Authorization": "Bearer <token>"}` in its headers pane.
- gRPC:
  - URL: `grpc.lvh.me:80` (h2c through Caddy) | `127.0.0.1:8081`
  - [v1/auth.proto](docs/proto/v1/auth.proto)
  - [v1/task.proto](docs/proto/v1/task.proto)
  - [v1/exchange.proto](docs/proto/v1/exchange.proto)
- PostgreSQL:
  - `postgres://user:myAwEsOm3pa55@w0rd@127.0.0.1:5432/db` (the port is `PG_PORT`)
- Worker (no port; `docker logs -f worker`). Send it a message:
  - `psql ... -c "INSERT INTO queue_messages (queue, body) VALUES ('example', '{\"text\": \"hi\"}')"`
- Mailpit (catches every email the worker sends; register a user to get one):
  - http://127.0.0.1:8025 (inbox and API) | SMTP on `127.0.0.1:1025`
- Jaeger (traces UI):
  - http://jaeger.lvh.me | http://127.0.0.1:16686

## Observability

Distributed tracing is provided by [OpenTelemetry](https://opentelemetry.io/). Spans are exported over OTLP/gRPC to a
collector — [Jaeger](https://www.jaegertracing.io/) in the docker stack.

- **Context propagation** — W3C `traceparent` + `baggage`, so a single trace spans every transport. REST and GraphQL use the
  [otelhttp](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp) middleware, and gRPC uses the
  [otelgrpc](https://github.com/open-telemetry/opentelemetry-go-contrib) stats handler.
- **Instrumented layers** — use cases and repositories are wrapped in tracing decorators (`*/tracing.go`), so a trace
  shows the full path: controller → usecase → repo / webapi.
- **No-op when disabled** — with `TRACING_ENABLED=false` a no-op tracer provider is installed, so instrumentation runs
  unconditionally with zero overhead.
- **Sampling** — `TRACING_SAMPLE_RATE` is a parent-based ratio. Use `1.0` locally; lower it (`0.1` / `0.01`) in
  production.
- Internal routes (`/healthz`, `/metrics`, `/docs`) are excluded from tracing — only the versioned API groups are
  instrumented.

Configuration (see [.env.example](.env.example)):

| Variable                | Default            | Description                       |
|-------------------------|--------------------|-----------------------------------|
| `TRACING_ENABLED`       | `false`            | Enable OpenTelemetry tracing      |
| `TRACING_OTLP_ENDPOINT` | `localhost:4317`   | OTLP/gRPC collector endpoint      |
| `TRACING_OTLP_INSECURE` | `true`             | Disable TLS for the OTLP exporter |
| `TRACING_SAMPLE_RATE`   | `0.1`              | Parent-based sampling ratio       |

## Project structure

### `cmd/app/main.go`

Configuration and logger initialization. Then the main function "continues" in
`internal/app/app.go`.

### `cmd/worker/main.go`

Configuration, then `RunWorker` in `internal/app/worker.go`: the queue consumer. It runs from the same
Docker image as the app (`/worker` instead of `/app`) and does not apply the schema.

### `cmd/openapi/main.go`

Prints the REST OpenAPI document without starting a server or touching the database.
`make openapi` writes it to `docs/openapi/v1.json`, and CI fails when that file is stale.

### `config`

The twelve-factor app stores config in environment variables (often shortened to `env vars` or `env`). Env vars are easy
to change between deploys without changing any code; unlike config files, there is little chance of them being checked
into the code repo accidentally; and unlike custom config files, or other config mechanisms such as Java System
Properties, they are a language- and OS-agnostic standard.

Config: [config.go](config/config.go)

Example: [.env.example](.env.example)

[docker-compose.yml](docker-compose.yml) uses `env` variables to configure services. `PG_PORT` (default `5432`) is the
host port the compose `db` service publishes, and `PG_URL` in `.env.example` is built from it. `ATLAS_DEV_URL` is read
only by `make sqlc`; the app does not use it.

Email is sent only by the worker, so none of these is required:

| Env var | Default | |
| --- | --- | --- |
| `APP_BASE_URL` | `http://localhost` | where the web client is served; email links point under it |
| `SMTP_HOST` | `localhost` | SMTP relay (`mailpit` in compose) |
| `SMTP_PORT` | `1025` | |
| `SMTP_USERNAME` | empty | empty turns auth off; set, it uses PLAIN, sent only over TLS or to localhost |
| `SMTP_PASSWORD` | empty | |
| `MAIL_FROM` | `Go Clean Template <noreply@example.com>` | sender address; keep `.env` values free of spaces, since both `make` and the shell read that file |

The exchange domain reads rates from the ExchangeRate-API open access endpoint:

| Env var | Default | |
| --- | --- | --- |
| `EXCHANGE_SERVICE_URL` | `https://open.er-api.com` | base URL of the rate provider; the integration tests point it at a stub |

#### `docs/openapi`

The REST OpenAPI document, exported from the Huma registrations by `make openapi`. It is the same
document Huma serves at `/openapi.json`, checked in so contract changes are reviewable and clients
can generate types from it. Don't edit it by hand.

#### `docs/proto`

Protobuf files. They are used to generate Go code for gRPC services.
The proto files are also used to generate documentation for gRPC services.
You don't need to correct anything by yourself.

### `integration-test`

Integration tests.
They are launched as a separate container, next to the application container.
Exchange-rate calls go to a Caddy stub ([integration-test/exchange-stub/Caddyfile](integration-test/exchange-stub/Caddyfile))
instead of open.er-api.com, so the suite never calls a third party. The stub serves the same recorded response
(`internal/repo/webapi/testdata/openerapi_latest_omr.json`) the webapi unit test reads, so the two cannot drift apart.

### `internal/app`

There is always one _Run_ function in the `app.go` file, which "continues" the _main_ function.

This is where all the main objects are created.
Dependency injection occurs through the "New ..." constructors (see Dependency Injection).
This technique allows us to layer the application using the [Dependency Injection](#dependency-injection) principle.
This makes the business logic independent from other layers.

Next, we start the server and wait for signals in _select_ for graceful completion.
If `app.go` starts to grow, you can split it into multiple files.

For a large number of injections, [wire](https://github.com/google/wire) can be used.

The `migrate.go` file applies the database schema at startup with [Atlas](https://atlasgo.io/).
The desired schema is defined declaratively in `internal/app/schema/*.hcl`, one file per table —
there are no migration files. On every boot Atlas diffs the directory against the app database and makes the
database match it: it creates what is missing and drops tables and columns the HCL does not declare. There is no
separate Atlas dev database, and the stack runs exactly one database.
It is included if an argument with the _migrate_ tag is specified.
For example:

```sh
go run -tags migrate ./cmd/app
```

### `internal/controller`

Server handler layer (MVC controllers). The template shows 3 servers:

- gRPC ([gRPC](https://grpc.io/) framework based on protobuf)
- REST API ([chi](https://github.com/go-chi/chi) router wrapped with the [Huma](https://github.com/danielgtaylor/huma) framework)
- GraphQL ([gqlgen](https://github.com/99designs/gqlgen), tasks and the exchange history)

Server routers are written in the same style:

- Handlers are grouped by area of application (by a common basis)
- For each group, its own router structure is created, the methods of which process paths
- The structure of the business logic is injected into the router structure, which will be called by the handlers

#### `internal/controller/worker`

Queue handlers for the worker binary. A queue message is an inbound request like an HTTP call, so a
handler has the same shape as one: decode the body into a `v1/request` DTO, call a use case with the
handler's `ctx`, and return its error. Registration is in `v1/router.go` (`w.Handle(queue, handler)`).
`v1/example.go` only logs the message. `v1/mail.go` delivers email from the `mail` queue; its body is
the envelope `entity.EncodeEmail` writes, decoded with `entity.DecodeEmail` instead of a request DTO,
because the use case that enqueues it must write the same shape.

#### `internal/controller/graphql`

GraphQL endpoint for tasks and the exchange history, mounted at `/graphql` on the REST router. The
schemas live in `schema/*.graphqls`, one per domain; `make graphql` regenerates `generated/` and
`model/` and updates the resolver signatures in `<domain>.resolvers.go`. `Task` and `TaskStatus` bind to the `entity` types
through `gqlgen.yml`. Resolvers authenticate the bearer token themselves, so introspection works
without one, and errors carry `extensions.code` (`UNAUTHENTICATED`, `BAD_USER_INPUT`,
`NOT_FOUND`, `FORBIDDEN`, `INTERNAL`).

#### `internal/controller/grpc`

Simple gRPC versioning.
For v2, we will need to add the `grpc/v2` folder with the same content.
Also add the `v2` folder to the proto files in `docs/proto`.
And in the file `internal/controller/grpc/router.go` add the line:

```go
{
    v1.NewAuthRoutes(app, u, l)
    v1.NewTaskRoutes(app, tk, l)
    v1.NewExchangeRoutes(app, e, l)
}

{
    v2.NewAuthRoutes(app, u, l)
    v2.NewTaskRoutes(app, tk, l)
    v2.NewExchangeRoutes(app, e, l)
}

reflection.Register(app)
```

#### `internal/controller/restapi`

Simple REST versioning.
For v2, we will need to add the `restapi/v2` folder with the same content.
And in the file `internal/controller/restapi/router.go` add the line:

```go
{
	v1.NewRoutes(api, e, u, tk, l)
}

{
	v2.NewRoutes(api, e, u, tk, l)
}
```

Huma generates the OpenAPI spec and serves `/docs`, `/openapi.json`, and `/openapi.yaml` — routes,
validation rules, and the BearerAuth security scheme all follow the registered input and output
structs, so there are no annotation comments to maintain. REST behavior notes against the
previous Fiber release:

- The request-count and duration metrics are `http_requests_total` and
  `http_request_duration_seconds`, labeled by `method`, `route` (the chi route pattern, or
  `unmatched`) and `code`; the fiberprometheus series names stopped with that dependency.
- Error bodies are RFC 9457 `application/problem+json` (`type`, `title`, `status`, `detail`, and
  `errors` for validation failures) instead of `{"error": "..."}`. Read `detail` for the message.
  Validation errors name the failing `location` but never echo the submitted value.
- Requests failing Huma's structural validation answer `422`, not the old `400`. That includes
  `GET /v1/tasks` query parameters: `limit` must be an integer of at least 1, `offset` at least 0,
  and `status` one of `todo`, `in_progress`, `done`. Out-of-range or unparsable values no longer
  fall back to the defaults.
- Unknown fields in a request body are ignored, as before.

For v2, register the v2 operations with the same `api` object in
`internal/controller/restapi/router.go`: the v2 paths (`/v2/...`) are plain chi routes in the same
Huma spec, and the `apiV2Group := chi.NewRouter() …` case from the previous release no longer applies.

### `internal/entity`

Entities of business logic (models) can be used in any layer.
There can also be methods, for example, for validation.

### `internal/usecase`

Business logic.

- Methods are grouped by area of application (on a common basis)
- Each group has its own structure
- One file - one structure

Repositories, webapi, and other business logic structures are injected into business logic structures
(see [Dependency Injection](#dependency-injection)).

#### `internal/repo/persistent`

A repository is an abstract storage (database) that business logic works with.
SQL queries live in `queries/*.sql`; [sqlc](https://github.com/sqlc-dev/sqlc) compiles them into typed Go in the same `queries` package.
sqlc cannot read HCL, so `make sqlc` first renders `internal/app/schema/*.hcl` to `queries/schema/schema.sql` with Atlas
in a throwaway Postgres container (`ATLAS_DEV_URL=docker://postgres/18/dev`) that exists only while the command runs,
so Docker is needed. Run `make sqlc` after changing the schema or a query.

`queue/` is the message queue the worker binary consumes (see [`pkg/worker`](#pkgworker)): `Send`,
`Read` and `Delete` over the `queue_messages` table.

#### `internal/repo/mailtmpl`

Renders transactional email from Go templates embedded in the binary: the HTML part with
`html/template` (which escapes user data), the subject and plain-text part with `text/template`.

- `templates/layout/` holds what every email shares: `layout.html.tmpl` (table markup, inline styles),
  `partials.html.tmpl` (`heading`, `paragraph`, `button`, `footnote`) and `layout.txt.tmpl`.
- An email is two files, `templates/<name>.html.tmpl` defining `content` and
  `templates/<name>.txt.tmpl` defining `subject` and `text`, executed against the `entity.Email`
  whose `Template()` returns `<name>`.
- Templates are parsed at startup, so a broken one stops the binary. A template that reads a field its
  struct lacks fails `make test`: the render test executes every email against a full and a zero-value
  fixture and compares it with `testdata/<name>.golden.{html,txt}` (`UPDATE_GOLDEN=1` rewrites them).

Sending is asynchronous: a use case enqueues on `entity.EmailQueue` with `repo.QueueRepo`, and the
worker's `mail` handler calls the `mail` use case, which renders and sends through
[`pkg/smtp`](#pkgsmtp). Delivery is at-least-once, so an email can arrive twice if the worker stops
between the relay accepting it and the queue row being deleted.

Adding an email:

1. Struct in `internal/entity/email.go` with `Template()` and `Recipient()`, and a case for it in
   `DecodeEmail`.
2. `templates/<name>.html.tmpl` and `templates/<name>.txt.tmpl`.
3. A full and a zero-value fixture in `mailtmpl_test.go`, then `UPDATE_GOLDEN=1 go test ./internal/repo/mailtmpl/`
   and review the golden files.
4. In the use case: `body, err := entity.EncodeEmail(e)`, then `queue.Send(ctx, entity.EmailQueue, body)`.

#### `internal/repo/webapi`

It is an abstract web API that business logic works with.
For example, it could be another microservice that business logic accesses via the REST API.
The package name changes depending on the purpose.

`exchange_openerapi.go` implements `repo.ExchangeWebAPI` with plain `net/http` (an `otelhttp`
transport, so the outbound call joins the trace). It decodes rates straight into `decimal.Decimal`
and maps the provider's `unsupported-code` error (sent with HTTP 200) to `entity.ErrUnsupportedCurrency`. Its unit test serves the recorded
response in `testdata/` from an `httptest` server.

### `pkg/worker`

Polls the registered queues and calls one handler per message. The queue itself is the
`queue_messages` table (`internal/app/schema/queue_messages.hcl`), read and written through
`internal/repo/persistent/queue`. One table holds every queue, told apart by its `queue` column, so a
queue exists as soon as something is sent to it.

- **Claiming.** A read takes up to `WORKER_BATCH_SIZE` due messages with `FOR UPDATE SKIP LOCKED` and,
  in the same statement, hides them until `now() + WORKER_VISIBILITY_TIMEOUT`. Several workers can
  run side by side without getting the same message at the same time.
- **At-least-once.** A message is deleted only after its handler returns `nil`. If the handler fails,
  the message comes back once its visibility timeout expires, so handlers must be idempotent.
  `worker.Message.ReadCount` tells a handler how many times it has been delivered.
- **Shutdown.** On SIGTERM it stops reading, waits up to `WORKER_SHUTDOWN_TIMEOUT` for in-flight
  handlers, then cancels their context.
- **Startup.** The app applies the schema, not the worker. A worker that starts first logs
  `relation "queue_messages" does not exist` until the app has run, then carries on.

Adding a queue:

1. Handler: `internal/controller/worker/v1/<name>.go`, registered in `v1/router.go`.
2. Producer: inject `repo.QueueRepo` into the use case and call `Send(ctx, "<name>", body)`.

| Env var | Default | |
| --- | --- | --- |
| `WORKER_POLL_INTERVAL` | `1s` | sleep after an empty read |
| `WORKER_VISIBILITY_TIMEOUT` | `30s` | how long a read message is hidden before redelivery |
| `WORKER_BATCH_SIZE` | `10` | messages per read |
| `WORKER_SHUTDOWN_TIMEOUT` | `10s` | grace period for in-flight handlers |

### `pkg/smtp`

An SMTP client on the standard library's `net/smtp`: one connection per message, STARTTLS when the
server offers it, PLAIN auth when a username is set. A message goes out as `multipart/alternative`
(text, then HTML, both quoted-printable), with a Q-encoded subject that cannot carry a header. It
implements `repo.Mailer`; another provider is another implementation of that interface.

### `pkg/tracing`

OpenTelemetry setup: configures a global `TracerProvider` with an OTLP/gRPC exporter and W3C trace-context + baggage
propagators. When tracing is disabled, a no-op provider is installed so instrumentation code runs unconditionally.
See [Observability](#observability).

### `web`

The React client. Routes are files in `web/src/routes/` (TanStack Router); server state goes through
TanStack Query. `web/src/api/schema.gen.ts` is generated from `docs/openapi/v1.json` by
`make web-api`, so a REST contract change shows up as a type error here. In development the Vite+
dev server proxies the API paths to the Go app, so the client and API share one origin and no CORS
is needed. `make web-check` formats, lints, type-checks and tests it.

The UI is built from [shadcn/ui](https://ui.shadcn.com/) components (`web/components.json`; the
generated components live in `web/src/components/ui/`). Add one with
`vp dlx shadcn@latest add <component>` from `web/`. Signed-in screens (`/tasks`, `/profile`) share a
sidebar layout; `/login` and `/register` use a centred card.

Tables are shadcn data tables on [TanStack Table](https://tanstack.com/table) and read their rows
over GraphQL (`/graphql`), so a list fetches only the columns it shows; forms and the profile use
REST. GraphQL operations are written with `graphql(...)` in `web/src/` and typed by
`make web-graphql` (GraphQL Code Generator) from the gqlgen schema. Every request, REST or GraphQL,
goes through TanStack Query. Forms use [TanStack Form](https://tanstack.com/form) with shadcn `Field`
components and a [Zod](https://zod.dev/) schema per domain (`web/src/components/form/`).

## Dependency Injection

In order to remove the dependence of business logic on external packages, dependency injection is used.

For example, through the New constructor, we inject the dependency into the structure of the business logic.
This makes the business logic independent (and portable).
We can override the implementation of the interface without making changes to the `usecase` package.

```go
package usecase

import (
// Nothing!
)

type Repository interface {
	Get()
}

type UseCase struct {
	repo Repository
}

func New(r Repository) *UseCase {
	return &UseCase{
		repo: r,
	}
}

func (uc *UseCase) Do() {
	uc.repo.Get()
}
```

It will also allow us to do auto-generation of mocks (for example with [go.uber.org/mock](https://go.uber.org/mock)) and
easily write unit tests.

> We are not tied to specific implementations in order to always be able to change one component to another.
> If the new component implements the interface, nothing needs to be changed in the business logic.

## Clean Architecture

### Key idea

Programmers realize the optimal architecture for an application after most of the code has been written.

> A good architecture allows decisions to be delayed to as late as possible.

### The main principle

Dependency Inversion (the same one from SOLID) is the principle of dependency injection.
The direction of dependencies goes from the outer layer to the inner layer.
Due to this, business logic and entities remain independent from other parts of the system.

So, the application is divided into 2 layers, internal and external:

1. **Business logic** (Go standard library).
2. **Tools** (databases, servers, message brokers, any other packages and frameworks).

![Clean Architecture](docs/img/layers-1.png)

**The inner layer** with business logic should be clean. It should:

- Not have package imports from the outer layer.
- Use only the capabilities of the standard library.
- Make calls to the outer layer through the interface (!).

The business logic doesn't know anything about Postgres or a specific web API.
Business logic has an interface for working with an _abstract_ database or _abstract_ web API.

**The outer layer** has other limitations:

- All components of this layer are unaware of each other's existence. How to call another from one tool? Not directly,
  only through the inner layer of business logic.
- All calls to the inner layer are made through the interface (!).
- Data is transferred in a format that is convenient for business logic (`internal/entity`).

For example, you need to access the database from HTTP (controller).
Both HTTP and database are in the outer layer, which means they know nothing about each other.
The communication between them is carried out through `usecase` (business logic):

```
    HTTP > usecase
           usecase > repository (Postgres)
           usecase < repository (Postgres)
    HTTP < usecase
```

The symbols > and < show the intersection of layer boundaries through Interfaces.
The same is shown in the picture:

![Example](docs/img/example-http-db.png)

Or more complex business logic:

```
    HTTP > usecase
           usecase > repository
           usecase < repository
           usecase > webapi
           usecase < webapi
           usecase > RPC
           usecase < RPC
           usecase > repository
           usecase < repository
    HTTP < usecase
```

### Layers

![Example](docs/img/layers-2.png)

### Clean Architecture Terminology

- **Entities** are structures that business logic operates on.
  They are located in the `internal/entity` folder.
  In MVC terms, entities are models.
- **Use Cases** is business logic located in `internal/usecase`.

The layer with which business logic directly interacts is usually called the _infrastructure_ layer.
These can be repositories `internal/repo/persistent`, external webapi `internal/repo/webapi`, any pkg, and other
microservices.
In the template, the _infrastructure_ packages are located inside `internal/repo`.

You can choose how to call the entry points as you wish. The options are:

- controller (in our case)
- delivery
- transport
- gateways
- entrypoints
- primary
- input

### Additional layers

The classic version
of [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html) was designed for
building large monolithic applications and has 4 layers.

In the original version, the outer layer is divided into two more, which also have an inversion of dependencies
to each other (directed inward) and communicate through interfaces.

The inner layer is also divided into two (with separation of interfaces), in the case of complex logic.

---

Complex tools can be divided into additional layers.
However, you should add layers only if really necessary.

### Alternative approaches

In addition to Clean architecture, _Onion architecture_ and _Hexagonal_ (_Ports and adapters_) are similar to it.
Both are based on the principle of Dependency Inversion.
_Ports and adapters_ are very close to _Clean Architecture_, the differences are mainly in terminology.

## Similar projects

- [https://github.com/bxcodec/go-clean-arch](https://github.com/bxcodec/go-clean-arch)
- [https://github.com/zhashkevych/courses-backend](https://github.com/zhashkevych/courses-backend)

## Useful links

- [The Clean Architecture article](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Twelve factors](https://12factor.net/ru/)
