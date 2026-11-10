---
title: Installation
description: Install the tools the template needs, clone it and set up your environment file.
sidebar:
  order: 1
---

## Prerequisites

| Tool                                               | Why you need it                                                                                                              |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Docker with Compose                                | Postgres, Valkey, MongoDB, NATS, Mailpit and Keycloak run in containers.                                                     |
| Go, at the version in `go.mod` (`go 1.27.3` today) | The app, the worker and the generator are Go binaries.                                                                       |
| tmux                                               | `make dev` runs every process in one tmux window.                                                                            |
| curl                                               | `make dev` polls the app's `/healthz` before it starts the worker.                                                           |
| jq                                                 | The first-request walkthrough in the tour reads the token from the login response with it.                                   |
| `protoc`                                           | `make run`, `make pre-commit` and the generators regenerate the gRPC code. `make bin-deps` does not install `protoc` itself. |
| The `vp` CLI (Vite+)                               | Builds and serves the React client in `web/`. It provisions the Node version in `web/.node-version` and bun.                 |

`make dev` checks that `tmux`, `docker`, `go`, `curl` and `vp` are on your `PATH` and stops with a
message naming the first one it cannot find.

## Get the code

```sh
git clone https://github.com/m7medvision/go-clean-template.git
cd go-clean-template
make bin-deps   # the Go tools declared in go.mod (mockgen, linters, ...)
make web-deps   # the web client's packages, from the frozen lockfile
```

## Create your environment file

Every Make target loads `.env` and exports it. When there is no `.env`, it falls back to
`.env.example`, so the defaults work without a copy. To change a value, copy the example first:

```sh
cp .env.example .env
```

Keep values free of spaces and quotes: the same file is read both by `make` and by a plain shell.
Every setting is described in the [configuration reference](/go-clean-template/reference/configuration/).

## Port clashes

The compose services publish their ports on your machine. If something else already holds one,
move it with its port variable. The matching URL follows, because it is built from the port:

| Service  | Variable        | Default | URL that follows it |
| -------- | --------------- | ------- | ------------------- |
| Postgres | `PG_PORT`       | `5432`  | `PG_URL`            |
| Valkey   | `VALKEY_PORT`   | `6379`  | `VALKEY_URL`        |
| MongoDB  | `MONGO_PORT`    | `27017` | `MONGO_URL`         |
| NATS     | `NATS_PORT`     | `4222`  | `NATS_URL`          |
| Keycloak | `KEYCLOAK_PORT` | `8180`  | `OIDC_ISSUER`       |

Set it in `.env`, or pass it on the command line for one run:

```sh
make dev PG_PORT=5433
```

Next, [run the stack](/go-clean-template/getting-started/running/).
