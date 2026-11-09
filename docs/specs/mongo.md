# Spec: MongoDB in the stack

Module id: `mongo` · first of two (see [README.md](README.md#mongodb)) · Depends on nothing ·
Status: **approved 2026-11-09; work starts after the Valkey PR (#52) merges**

Paths below are checked against `feat/valkey` at `25a746b` on 2026-11-08.

## Objective

MongoDB becomes a required part of the stack the app runs on, ready for `task-attributes`, its only
user. The template shows that some data can live in a document store instead of Postgres, behind a
port, without the use cases or transports knowing which. This module adds no behaviour a user sees.
When it is done:

- the app refuses to start without `MONGO_URL`, the way it refuses without `VALKEY_URL`;
- the app connects and pings at startup, fails with a clear error when it cannot, and disconnects
  when it stops;
- every Mongo command is traced at the client level (otelmongo), so no repository carries tracing
  code;
- every way of running the app has a Mongo: `make dev`, `make compose-up`, `make compose-up-all`,
  `make compose-up-integration-test`, and CI's integration job;
- the worker runs exactly as before, without `MONGO_URL`.

**Who it is for:** a developer starting from the template who needs a place for data whose shape
changes every week; and an operator, who learns from the release notes and
`docs/configuration.md` that the app now needs a Mongo.

## Tech stack

| Piece | Version | Why |
| --- | --- | --- |
| Server | `mongo:8` line, exact tag pinned in the plan | latest stable, pinned like `postgres:18.6-alpine` |
| Client | `go.mongodb.org/mongo-driver/v2`, pinned in the plan | the official pure-Go driver; the build is `CGO_ENABLED=0` |
| Tracing | `go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo` | a span per command on the global tracer provider, as otelpgx does for SQL and valkeyotel for Valkey |

A single node, no replica set: `task-attributes` needs no multi-document transactions.

## Configuration

| Env var | Default | Read by | |
| --- | --- | --- | --- |
| `MONGO_URL` | required by the app | the app | `mongodb://localhost:${MONGO_PORT}/app` in `.env.example`, `mongodb://mongo:27017/app` in compose. The database is the URL's path |
| `MONGO_PORT` | `27017` | compose, `.env.example` | the host port the compose `mongo` service publishes; `MONGO_URL` follows it, as `PG_URL` follows `PG_PORT` |

`config.Mongo` has one field, `URL string \`env:"MONGO_URL"\``, with no `required` tag: the worker
parses the same config and has no Mongo. `app.Run` checks it and returns `ErrNoMongoURL`, beside
`ErrNoValkeyURL` (`internal/app/app.go:22`).

## Project structure

| File | Change |
| --- | --- |
| `pkg/mongo/mongo.go` | new: `New(ctx, url) (*mongo.Database, func(context.Context) error, error)`: parse the URL, build the traced client, ping, return the URL's database and a disconnect |
| `pkg/mongo/mongo_test.go` | new: a bad URL, a URL without a database and an unreachable address fail, without Docker |
| `config/config.go` | the `Mongo` section |
| `internal/app/app.go` | `ErrNoMongoURL`; `mongo.New` after Valkey, disconnected when `Run` returns; the database in `deps` (with its first user, in `task-attributes`) |
| `internal/app/app_test.go` | `Run` without `MONGO_URL` returns `ErrNoMongoURL` before it opens Postgres |
| `docker-compose.yml` | the `mongo` service with a `mongosh --eval 'db.runCommand({ping:1})'` healthcheck and a volume; `app` waits for it; `MONGO_URL` in the backend environment |
| `docker-compose-integration-test.yml` | `MONGO_URL` for the `integration-test` service |
| `Makefile` | `mongo` in `COMPOSE_SERVICES` |
| `.env.example` | `MONGO_PORT`, `MONGO_URL` |
| `.github/workflows/ci.yml` | the integration job runs Mongo with `docker run` and waits for the ping |
| `integration-test/mongo_test.go` | new: `mongo.New` against the stack's Mongo, a round trip on a `_test` collection |
| `docs/configuration.md`, `docs/development.md`, `docs/architecture.md`, `README.md`, `CLAUDE.md` | Mongo in the config table, the service tables, the Packages table, the front page's stack line, and "the app needs Mongo" |

## Code style

The repo's conventions; no comments unless one is important. The driver is imported as `mgo`,
since ours is `mongo` too.

```go
func New(ctx context.Context, url string) (*mgo.Database, func(context.Context) error, error) {
	cs, err := connstring.ParseAndValidate(url)
	if err != nil {
		return nil, nil, fmt.Errorf("mongo - New - connstring.Parse: %w", err)
	}

	if cs.Database == "" {
		return nil, nil, ErrNoDatabase
	}

	client, err := mgo.Connect(options.Client().ApplyURI(url).SetMonitor(otelmongo.NewMonitor()))
	if err != nil {
		return nil, nil, fmt.Errorf("mongo - New - mgo.Connect: %w", err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)

		return nil, nil, fmt.Errorf("mongo - New - ping: %w", err)
	}

	return client.Database(cs.Database), client.Disconnect, nil
}
```

## Testing strategy

- **Unit (`make test`, no Docker):** `pkg/mongo` fails on an unparsable URL, a URL without a
  database and an address nothing listens on (short server-selection timeout); `internal/app`
  returns `ErrNoMongoURL` with an empty URL. Table-driven, `t.Parallel()`.
- **Integration (`make compose-up-integration-test`, and CI):** insert, read and drop in the
  `_test` collection. The rest of the suite passing proves the app started.
- **By hand, once:** the startup ping span shows in Jaeger.

## Boundaries

- **Always:** `make format`, `make linter-golangci`, `make test` before each commit; pinned versions;
  the docs above in the same commit series.
- **Ask first:** a replica set or transactions; Mongo in the worker; a second user of Mongo beyond
  `task-attributes`; changes to CI outside the integration job.
- **Never:** a `required` tag on `MONGO_URL`; the driver imported by `usecase` or a controller; an
  Atlas dev database or a second Postgres; credentials committed outside `.env.example`'s
  development defaults.

## Success criteria

1. `app.Run` with an empty `MONGO_URL` returns `ErrNoMongoURL`, before it opens Postgres.
2. With `MONGO_URL` pointing at nothing, the app exits at startup with an error naming `mongo.New`.
3. `make dev`, `make compose-up-all` and `make compose-up-integration-test` start Mongo, and the app
   reaches it.
4. CI's integration job is green with Mongo in it.
5. The worker starts with no `MONGO_URL` set.
6. `make format`, `make linter-golangci` and `make test` pass; the docs say what changed.

## Decided (conversation of 2026-11-08)

- Mongo is required by the app, like Valkey, and unknown to the worker.
- No new domain: its first user is the `task` domain's attributes (`task-attributes`).
- Starts after the Valkey PR.
- Treated like Postgres (2026-11-09): compose sets `MONGO_INITDB_ROOT_USERNAME` and
  `MONGO_INITDB_ROOT_PASSWORD`, and `MONGO_URL` carries them (`mongodb://user:pass@mongo:27017/app?authSource=admin`),
  as `PG_URL` carries Postgres's.
- **Breaking release** (2026-11-09): `feat(mongo)!:` with `BREAKING CHANGE: the app needs MONGO_URL
  and a MongoDB`.

## Open questions

None. One PR for `mongo` and `task-attributes`, one commit series per module, as Valkey did
(recommended, spec approved 2026-11-09).
