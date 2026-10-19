# Development

Running the template locally, testing it, and running the full stack. Drive everything through the
`Makefile` (`make help` lists every target). The targets load `.env` (or `.env.example`) and pass flags
that CI uses too.

## Dev mode

`make dev` runs everything in one [tmux](https://github.com/tmux/tmux) window. Postgres and Mailpit run
in docker. The app and the worker run on the host under [air](https://github.com/air-verse/air) and
rebuild when you save a file. Tracing is off and Jaeger is not started. It needs `tmux`, `curl` and the
Vite+ `vp` CLI; run `make web-deps` once first.

```sh
make dev       # attach; run it again to re-attach after detaching (prefix + d)
make dev-down  # close the tmux session and stop Postgres and Mailpit
```

The four panes are:

- **app**: `.air.app.toml`, built with `-tags migrate`. It rebuilds on `.go`, `.hcl` and `.tmpl`
  changes, so saving a schema file re-syncs the database.
- **worker**: `.air.worker.toml`. It starts once the app answers `/healthz`, because only the app
  applies the schema.
- **web**: the Vite+ dev server on http://localhost:3000.
- **db + mailpit**: the container logs.

If something else on the host already listens on 5432, set `PG_PORT` in `.env` (for example
`PG_PORT=5433`). It moves the published port of the compose `db` service and the port in `PG_URL`
together. Without it, the app connects to whatever already owns 5432, and the startup schema apply
will try to make that database match this schema.

The same pieces without tmux, one terminal each:

```sh
make compose-up   # Postgres, and Mailpit to catch email (inbox on http://127.0.0.1:8025)
make run          # the app, applying the schema at startup
make run-worker   # the queue worker
make web-dev      # the web client on http://localhost:3000
```

`make run` regenerates the protobuf code first, so it needs `protoc` on `PATH`. `make bin-deps`
installs the other tools (`mockgen`, linters, `atlas`).

## Local services

| Service | URL |
| --- | --- |
| Web client | http://localhost:3000 |
| REST, GraphQL, API docs | http://127.0.0.1:8080 (`/v1`, `/graphql`, `/docs`); see [api.md](api.md) |
| gRPC | `127.0.0.1:8081` |
| Postgres | `127.0.0.1:${PG_PORT}`; credentials in `PG_URL` in [`.env.example`](../.env.example) |
| Mailpit | http://127.0.0.1:8025 (inbox and API), SMTP on `127.0.0.1:1025` |
| Jaeger | http://127.0.0.1:16686 (full stack only) |

The worker has no port; with the full stack, `docker logs -f worker` follows it.

## Tests

```sh
make test                         # unit tests, with -race
make compose-up-integration-test  # DB, app, worker, Mailpit, then the integration suite
```

The integration suite runs inside the docker network and reaches the app by container name, so it
fails on the host. Use the compose target, not `make integration-test`. Exchange-rate calls go to a
Caddy stub ([integration-test/exchange-stub/Caddyfile](../integration-test/exchange-stub/Caddyfile))
instead of open.er-api.com. The stub serves the same recorded response
(`internal/repo/webapi/testdata/openerapi_latest_omr.json`) that the webapi unit test reads, so the two
cannot drift apart.

Before pushing, `make pre-commit` runs the generators, format, lint, and the Go and web tests.

## Full stack behind Caddy

```sh
make compose-up-all
```

This runs the app, the worker, Postgres, Mailpit, Jaeger and a [Caddy](https://caddyserver.com/)
reverse proxy, built from [`caddy/Dockerfile`](../caddy/Dockerfile) and configured in
[`caddy/Caddyfile`](../caddy/Caddyfile). The same image builds `web/` with Vite+ and serves it on
`app.lvh.me`. The paths the Go app owns (`/v1`, `/graphql`, `/docs`, `/openapi.*`, `/schemas`,
`/healthz`, `/metrics`) go to the app; everything else is the web client, with a
Content-Security-Policy and history fallback to `index.html`. Client and API share one origin, so there
is no CORS.

- [caddy-waf](https://github.com/fabriziosalmi/caddy-waf) inspects every request to `app.lvh.me` with
  its bundled rule set and answers `403` to a match. It is licensed AGPL-3.0, unlike this template.
- [Souin](https://github.com/darkweak/souin) caches the public documents (`/docs`, `/openapi.*`,
  `/schemas/*`) in memory and reports a `Cache-Status` header. API responses are per caller and are
  never cached.

| Service | URL |
| --- | --- |
| Web client, REST, GraphQL, docs | http://app.lvh.me |
| gRPC | `grpc.lvh.me:80` (h2c) |
| Jaeger | http://jaeger.lvh.me |

`make compose-down` tears it down.
