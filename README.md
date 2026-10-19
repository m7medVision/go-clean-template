![Go Clean Template](docs/img/logo.svg)

# Go Clean template

A Go service laid out with Clean Architecture: three domains, served over REST, gRPC and GraphQL from
one use-case layer, with a background worker and a React client.

[![Go version](https://img.shields.io/github/go-mod/go-version/m7medVision/go-clean-template)](go.mod)
[![License](https://img.shields.io/github/license/m7medVision/go-clean-template)](LICENSE)

Forked from [evrone/go-clean-template](https://github.com/evrone/go-clean-template).

![The /exchange page: a currency conversion form and the history table](docs/img/exchange.png)

## What you get

- **Three transports, one use-case layer.** REST ([chi](https://github.com/go-chi/chi) +
  [Huma](https://github.com/danielgtaylor/huma), OpenAPI generated from the code), gRPC, and GraphQL
  ([gqlgen](https://github.com/99designs/gqlgen)).
- **Three domains.** User auth (JWT), tasks with a status state machine, and currency exchange
  against an outbound HTTP API.
- **Postgres and nothing else.** Typed queries with [sqlc](https://sqlc.dev/), a declarative
  [Atlas](https://atlasgo.io/) schema applied at startup, and a job queue in one ordinary table.
- **A worker binary.** It consumes that queue and sends email from templates embedded in the binary.
- **A React client.** TanStack Router, Query, Table and Form, typed from the OpenAPI document and the
  GraphQL schema.
- **OpenTelemetry tracing** through every layer, Prometheus metrics, and a Caddy front end with a WAF.
- **Generated code wherever it can be:** SQL, mocks, protobuf, GraphQL, OpenAPI, frontend types.

## Clean Architecture

The code is split into layers, and **dependencies only point inward**:

![Clean Architecture](docs/img/layers-1.png)

```
controller  →  usecase  →  repo (interfaces)  →  entity
```

- `internal/entity`: domain types and rules. Imports nothing from this module.
- `internal/usecase`: business logic. Talks to storage and outside APIs only through interfaces.
- `internal/repo`: Postgres, outbound HTTP and email, behind those interfaces.
- `internal/controller`: REST, gRPC, GraphQL and the queue worker. Each one only decodes, calls a use
  case, and encodes. Controllers never import each other.
- `pkg/`: reusable infrastructure with no domain knowledge. Never imports `internal/`.

These are rules, not suggestions: a change that breaks one is not merged.
[docs/architecture.md](docs/architecture.md) lists them with where each kind of code goes, and
[docs/request-flow.md](docs/request-flow.md) follows one request through every layer.

## Quick start

Needs Docker, Go, `tmux`, `curl` and the [Vite+](https://viteplus.dev/) `vp` CLI.

```sh
cp .env.example .env
make web-deps   # frontend dependencies, once
make dev        # Postgres + Mailpit in docker; app, worker and web client with hot reload
```

Then open:

- http://localhost:3000: the web client (register a user to start)
- http://127.0.0.1:8080/docs: the REST API reference
- http://127.0.0.1:8080/graphql/playground: GraphQL
- http://127.0.0.1:8025: Mailpit, which catches the welcome email

If Postgres already runs on the host's port 5432, set `PG_PORT=5433` in `.env` first.

## Docs

- [Architecture](docs/architecture.md): the rules, where code goes, packages, Clean Architecture
- [A request, end to end](docs/request-flow.md): one request through every layer, file by file
- [API](docs/api.md): every endpoint on every transport, errors, versioning
- [Development](docs/development.md): dev mode, tests, the full stack behind Caddy
- [Configuration](docs/configuration.md): every environment variable, tracing
- [Queue, worker and email](docs/worker.md)
- [Web client](docs/web.md)

## License

[MIT](LICENSE)
