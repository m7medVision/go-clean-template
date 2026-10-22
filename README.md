![Go Clean Template](docs/img/logo.svg)

# Go Clean template

A Go service laid out with Clean Architecture: three domains behind one use-case layer that every
transport shares.

[![Go version](https://img.shields.io/github/go-mod/go-version/m7medVision/go-clean-template)](go.mod)
[![License](https://img.shields.io/github/license/m7medVision/go-clean-template)](LICENSE)
[![Release](https://img.shields.io/github/v/release/m7medVision/go-clean-template)](https://github.com/m7medVision/go-clean-template/releases) <!-- scaffold:self -->

Forked from [evrone/go-clean-template](https://github.com/evrone/go-clean-template).

## What you get

- **One use-case layer under every transport:**
  - REST ([chi](https://github.com/go-chi/chi) + [Huma](https://github.com/danielgtaylor/huma),
    OpenAPI generated from the code).
  - gRPC. <!-- scaffold:grpc -->
  - GraphQL ([gqlgen](https://github.com/99designs/gqlgen)). <!-- scaffold:graphql -->
- **Three domains.** User auth (JWT), tasks with a status state machine, and currency exchange
  against an outbound HTTP API.
<!-- scaffold:passkeys:begin -->
- **Passkeys alongside passwords.** Add one from the profile, sign in with it from a button or the
  browser's autofill; each sign-in challenge works once.
<!-- scaffold:passkeys:end -->
- **Roles** carried in the auth token and checked in the use cases.
<!-- scaffold:sso:begin -->
- **SSO.** Sign in through any OpenID Connect provider (Keycloak ships ready in dev, with an admin
  and a user), with the provider's roles mapped to the app's.
<!-- scaffold:sso:end -->
- **Postgres and nothing else.** Typed queries with [sqlc](https://sqlc.dev/) and a declarative
  [Atlas](https://atlasgo.io/) schema applied at startup.
  The job queue is one ordinary table too. <!-- scaffold:queue -->
<!-- scaffold:queue:begin -->
- **A worker binary.** It consumes that queue, starting from an example handler.
  It sends email from templates embedded in the binary. <!-- scaffold:email -->
<!-- scaffold:queue:end -->
<!-- scaffold:web:begin -->
- **A React client.** TanStack Router, Query, Table and Form, typed from the OpenAPI document and the
  GraphQL schema.
<!-- scaffold:web:end -->
- **OpenTelemetry tracing** through every layer, Prometheus metrics, and a Caddy front end with a WAF.
- **Generated code wherever it can be:** SQL, mocks,
  protobuf, <!-- scaffold:grpc -->
  GraphQL, <!-- scaffold:graphql -->
  frontend types, <!-- scaffold:web -->
  and the OpenAPI document.

## Clean Architecture

The code is split into layers, and **dependencies only point inward**:

![Clean Architecture](docs/img/layers-1.png)

These are rules, not suggestions: a change that breaks one is not merged.
[docs/architecture.md](docs/architecture.md) lists them with where each kind of code goes, and
[docs/request-flow.md](docs/request-flow.md) follows one request through every layer.

## Quick start

Needs Docker, Go, `tmux` and `curl`.
The web client also needs the [Vite+](https://viteplus.dev/) `vp` CLI, and its dependencies installed once with `make web-deps`. <!-- scaffold:web -->

```sh
cp .env.example .env
make dev        # the app's dependencies in docker, everything else with hot reload
```

Then open:

- <http://localhost:3000>: the web client, where you register a user. <!-- scaffold:web -->
  **Sign in with SSO** as `alice` / `alice` to try the admin role. <!-- scaffold:sso -->
- <http://127.0.0.1:8080/docs>: the REST API reference
- <http://127.0.0.1:8080/graphql/playground>: GraphQL <!-- scaffold:graphql -->
- <http://127.0.0.1:8025>: Mailpit, which catches the email the app sends <!-- scaffold:email -->

If Postgres already runs on the host's port 5432, set `PG_PORT=5433` in `.env` first.

<!-- scaffold:self:begin -->
## Start your own project

Generate a service with only the features you pick (REST always, plus gRPC, GraphQL, the web
client, passkeys, SSO, the queue and email), without cloning this repository:

```sh
go run github.com/m7medvision/go-clean-template/cmd/create@latest
```

Each [release](https://github.com/m7medVision/go-clean-template/releases) also has the presets
ready-made as tarballs. See [docs/scaffold.md](docs/scaffold.md).

<!-- scaffold:self:end -->
## Docs

- [Architecture](docs/architecture.md): the rules, where code goes, packages, Clean Architecture
- [A request, end to end](docs/request-flow.md): one request through every layer, file by file
- [API](docs/api.md): every endpoint on every transport, errors, versioning
- [Development](docs/development.md): dev mode, tests, the full stack behind Caddy
- [Configuration](docs/configuration.md): every environment variable, tracing
- [Passkeys](docs/passkeys.md): the WebAuthn ceremonies, replay protection, relying-party setup <!-- scaffold:passkeys -->
- [SSO and roles](docs/sso.md): OpenID Connect sign-in, the dev Keycloak, role mapping, other providers <!-- scaffold:sso -->
- [Queue and worker](docs/worker.md) <!-- scaffold:queue -->
- [Web client](docs/web.md) <!-- scaffold:web -->
- [Scaffolding a project](docs/scaffold.md): `make scaffold` generates a new service with only the features you pick <!-- scaffold:self -->
- [Releases](docs/releasing.md): how versions are named from commits and what each release ships <!-- scaffold:self -->

## License

[MIT](LICENSE)
