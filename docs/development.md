# Development

Running the template locally, testing it, and running the full stack. Drive everything through the
`Makefile` (`make help` lists every target). The targets load `.env` (or `.env.example`) and pass flags
that CI uses too.

## Dev mode

`make dev` runs everything in one [tmux](https://github.com/tmux/tmux) window. The compose services
in `DEV_SERVICES` (see the `Makefile`) run in docker:

- `db`: Postgres.
- `valkey`: [Valkey](https://valkey.io/), which the app needs to start. It keeps no data on disk, so
  a restart empties it.
- `mongo`: [MongoDB](https://www.mongodb.com/), which the app needs to start. Its data is in the
  `mongo_data` volume; compose creates the root user `user` / `mongoPa55w0rd`, which `MONGO_URL`
  carries.
- `nats`: [NATS](https://nats.io/) with JetStream, the worker's queue, which the worker needs to
  start. Its streams are kept on the `nats_data` volume.
- `mailpit`: [Mailpit](https://mailpit.axllent.org/), which catches email.
- `keycloak`: the SSO provider.

The Go binaries run on the host under [air](https://github.com/air-verse/air) and rebuild when you
save a file. Tracing is off and Jaeger is not started. It needs `tmux` and `curl`.
The web pane also needs the Vite+ `vp` CLI; run `make web-deps` once first.

```sh
make dev       # attach; run it again to re-attach after detaching (prefix + d)
make dev-down  # close the tmux session and stop the containers
```

The panes are:

- **app**: `.air.app.toml`, built with `-tags migrate`. It rebuilds on `.go`, `.sql` and `.tmpl`
  changes, so a new migration is applied on the restart.
- **worker**: `.air.worker.toml`. It starts once the app answers `/healthz`, because only the app
  applies the schema. It runs the projections the app's reads come from, so the app needs it.
- **web**: the Vite+ dev server on http://localhost:3000.
- **the compose services**, titled after them (`db + …`): their container logs.

`make dev` sets `APP_BASE_URL` to the Vite origin, so the links the app builds from it come back to
the dev server.
Sign in with SSO as `alice` / `alice` (admin) or `bob` / `bob`; see
[sso.md](sso.md#keycloak-in-development).

If something else on the host already listens on 5432, set `PG_PORT` in `.env` (for example
`PG_PORT=5433`). It moves the published port of the compose `db` service and the port in `PG_URL`
together. Without it, the app connects to whatever already owns 5432, and the startup schema apply
will try to make that database match this schema. Set `VALKEY_PORT` the same way when 6379 is taken
(for example `VALKEY_PORT=6380`): it moves the compose `valkey` port and the port in `VALKEY_URL`
together.

The same pieces without tmux, one terminal each:

```sh
make compose-up   # the compose services in COMPOSE_SERVICES (DEV_SERVICES without the dev-only ones)
make run          # the app, applying the schema at startup
```

```sh
make web-dev      # the web client on http://localhost:3000
```

With the app and the worker running, `make web-e2e` runs the browser test (see
[web.md](web.md#end-to-end-test)).

```sh
make run-worker   # the worker: projections and the NATS topics, once the app has applied the schema
```

`make run` alone is not enough: without the worker the read models never catch up, and every write
answers `503` after `CQRS_CONSISTENCY_TIMEOUT` ([cqrs.md](cqrs.md#reads-after-writes)).
`make run` regenerates the protobuf code first, so it needs `protoc` on `PATH`.
`make bin-deps` installs the tools the targets run (`mockgen`, linters).

## Local services

| Service | URL |
| --- | --- |
| Web client | http://localhost:3000 |
| REST and the API docs | http://127.0.0.1:8080 (`/v1`, `/docs`); see [api.md](api.md) |
| GraphQL | http://127.0.0.1:8080/graphql; see [api.md](api.md#graphql) |
| gRPC | `127.0.0.1:8081` |
| Postgres | `127.0.0.1:${PG_PORT}`; credentials in `PG_URL` in [`.env.example`](../.env.example) |
| Valkey | `127.0.0.1:${VALKEY_PORT}`, and `valkey:6379` (alias `valkey.lvh.me`) inside the compose network; no UI |
| MongoDB | `127.0.0.1:${MONGO_PORT}`, and `mongo:27017` (alias `mongo.lvh.me`) inside the compose network; no UI (`docker compose exec mongo mongosh -u user -p mongoPa55w0rd`) |
| NATS | `127.0.0.1:${NATS_PORT}`, and `nats:4222` (alias `nats.lvh.me`) inside the compose network; monitoring on `:8222` inside it |
| Mailpit | http://127.0.0.1:8025 (inbox and API), SMTP on `127.0.0.1:1025` |
| Keycloak | http://keycloak.lvh.me:8180 (realm `app`; admin console `admin` / `admin`); see [sso.md](sso.md#keycloak-in-development) |
| Jaeger | http://127.0.0.1:16686 (full stack only) |

The worker has no port; with the full stack, `docker compose logs -f worker` follows it.

## Tests

```sh
make test                         # unit tests, with -race
make compose-up-integration-test  # the compose stack, then the integration suite
```

The integration suite runs inside the docker network and reaches the app by its compose service name, so it
fails on the host. Use the compose target, not `make integration-test`. Exchange-rate calls go to a
Caddy stub ([integration-test/exchange-stub/Caddyfile](../integration-test/exchange-stub/Caddyfile))
instead of open.er-api.com. The stub serves the same recorded response
(`internal/repo/webapi/testdata/openerapi_latest_omr.json`) that the webapi unit test reads, so the two
cannot drift apart.

```sh
make schema-roundtrip                   # every migration up, down to zero, up again; none merged edited
make schema-roundtrip BASE_REF=HEAD~1   # compare against any other branch or commit
```

`scripts/schema-roundtrip.sh` first fails if a migration that exists at the base ref was edited,
renamed or removed: a merged migration is never changed, a change is a new file. Then it builds
`cmd/migrate` and, on a throwaway Postgres container it starts and removes, applies every migration,
rolls them all back (`down-to 0`) and applies them again, so a `Down` that does not undo its `Up`
fails. It never touches `PG_URL`. It needs `docker` and `go` on `PATH`; with `ROUNDTRIP_PG_URL` set
it uses that database instead of a container, and refuses one that already has migrations applied.
In CI it runs on pull requests that change a migration or the migration code, against the PR's base
branch (the parent branch, for a stacked PR), on the Postgres `bmizerany/setup-postgres` starts,
like every other job that needs one.

Migrations by hand, on `PG_URL`'s database:

```sh
make migrate-new name=add_archived_at   # internal/app/migrations/<timestamp>_add_archived_at.sql
make migrate-status
make migrate-up
make migrate-down version=0             # roll back to a version; 0 empties the database
```

The domain and use-case generator, `tools/gen`, is its own module, so `make test` does not reach it:
`make gen-check` and `make gen-e2e` test it ([generator.md](generator.md#goldens-and-checks)).

Before pushing, `make pre-commit` runs the generators, format, lint and the tests.

## CI

`.github/workflows/ci.yml` runs on every pull request and on every push to `master`, in stages:

| Stage | Jobs | Runs when |
| --- | --- | --- |
| 1 | `changes` | always; sorts the diff into the path filters the later jobs read |
| 2 | `lint`: yamllint, hadolint, dotenv-linter (PRs only, on added lines), golangci-lint and govulncheck (backend) | always |
| 3 | `unit-tests` (backend, also the OpenAPI drift check), `sqlc-generated` (sqlc), `schema-roundtrip` (schema, PRs only), side by side | after `lint` passes |
| 3 | `web` (web): regenerates the client's types (failing on drift), checks and builds it, beside the other stage 3 jobs | after `lint` passes |
| 3 | `gen-check` (backend): `make gen-check`, the generator's lint and tests | after `lint` passes |
| 4 | `integration-tests` | backend, after `unit-tests` passes |
| 4 | `gen-e2e` (backend): `make gen-e2e`, which generates a domain and use cases and lints and tests them | after `gen-check` passes |
| 5 | `ci-ok` | always; fails if any job failed or was cancelled |

`ci-ok` is the check to require in branch protection: a job skipped because its paths did not change
counts as a pass. Each path filter lists the paths that are safe to skip, so a new
directory runs everything until someone adds it to a skip list; add one only when no file under it
can change that side's result. A new push to a PR cancels the run it supersedes.

`unit-tests` restores Go's build cache from the latest `master` run, so `go test` reruns only the
packages whose code, dependencies or test inputs changed.

## Full stack behind Caddy

```sh
make compose-up-all
```

This runs every compose service in `docker-compose.yml`, the app and Jaeger among them, behind a
[Caddy](https://caddyserver.com/) reverse proxy, built from [`caddy/Dockerfile`](../caddy/Dockerfile) and configured in
[`caddy/Caddyfile`](../caddy/Caddyfile). It serves `app.lvh.me`, where the paths the Go app owns go
to the app: `/v1`, `/graphql`, `/docs`, `/openapi.*`, `/schemas`, `/healthz` and `/metrics`.
The same image builds `web/` with Vite+ and serves it for every other path, with a
Content-Security-Policy and history fallback to `index.html`. Client and API share one origin, so
there is no CORS.

- [caddy-waf](https://github.com/fabriziosalmi/caddy-waf) inspects every request to `app.lvh.me` with
  its bundled rule set and answers `403` to a match. It is licensed AGPL-3.0, unlike this template.
- [Souin](https://github.com/darkweak/souin) caches the public documents (`/docs`, `/openapi.*`,
  `/schemas/*`) in memory and reports a `Cache-Status` header. API responses are per caller and are
  never cached.
- Compose pins `app_network` to `172.30.0.0/24` and gives Caddy `172.30.0.10`, the app's
  `TRUSTED_PROXY`, so the app sees the client behind Caddy
  ([configuration.md](configuration.md#the-client-address)).

| Service | URL |
| --- | --- |
| The app, behind Caddy | http://app.lvh.me |
| gRPC | `grpc.lvh.me:80` (h2c) |
| Jaeger | http://jaeger.lvh.me |

`make compose-down` tears it down.
