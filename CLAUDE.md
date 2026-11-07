# go-clean-template

Go clean-architecture reference service with CQRS and event sourcing (`docs/cqrs.md`). Three domains
(`user`, `task`, `exchange`) exposed from one shared set of commands and queries over each of these
transports:

- REST — chi + Huma v2.
- gRPC.
- GraphQL via gqlgen at `/graphql` on the REST port, for tasks and the exchange history; GraphiQL
  at `/graphql/playground` when `DOCS_ENABLED`.

A second binary, `cmd/worker`, runs the projections that keep the read models up to date and
consumes a minimal Postgres queue (the `queue_messages` table); see **CQRS and event sourcing** and
**Queue and worker** below. The app needs it running: every write waits for the projections.
Module path: `github.com/m7medvision/go-clean-template`. The Go version is declared in `go.mod`.
A React client (TanStack Router + Query, built with Vite+) lives in `web/`; see **Frontend** below.

## Commands — drive everything through the Makefile

The `Makefile` loads `.env` (falling back to `.env.example`) and exports it into every target, and
several targets carry flags that matter. Running the underlying tool directly means running it with
different settings than CI. Use the target, not the tool.

| Task | Use this | Not this |
| --- | --- | --- |
| Install tool binaries (`mockgen`, linters, `atlas`) | `make bin-deps` | `go install ...` |
| Unit tests | `make test` | `go test ./...` — the target adds `-race -covermode atomic` and scopes to `./config/... ./internal/... ./pkg/...` |
| Integration tests | `make compose-up-integration-test` | `make integration-test` — see below |
| Lint | `make linter-golangci` | `golangci-lint run` |
| Format | `make format` | `gofmt` — the target runs `go fix`, `gofumpt`, and `gci` with the repo's import grouping |
| Regenerate mocks | `make mock` | `mockgen ...` |
| Regenerate protobuf | `make proto-v1` | `protoc ...` |
| Regenerate SQL query code | `make sqlc` | `sqlc generate` — the target first renders the HCL schema to SQL with Atlas (needs Docker for `ATLAS_DEV_URL`), then runs the sqlc pinned in `go.mod` |
| Regenerate the REST OpenAPI document | `make openapi` | `go run ./cmd/openapi` — the target writes through a temp file so a failed run never truncates `docs/openapi/v1.json` |
| Install frontend dependencies | `make web-deps` | `bun install` / `vp install` — the target uses the frozen lockfile |
| Regenerate frontend REST types | `make web-api` | `npx openapi-typescript ...` — the target refreshes `docs/openapi/v1.json` first |
| Regenerate frontend GraphQL types | `make web-graphql` | `npx graphql-codegen` — reads the gqlgen schema and the operations in `web/src/` |
| Run the frontend dev server | `make web-dev` | `vp dev` — the target exports `HTTP_PORT`, which the Vite proxy targets |
| Format / lint / test the frontend | `make web-format`, `make web-check` | `vp check`, `vp test`, `tsc`, `eslint`, `prettier` |
| Build the frontend | `make web-build` | `vp build` |
| Regenerate GraphQL | `make graphql` | `go tool gqlgen ...` — `gqlgen.yml` paths resolve against the working directory, so the target runs gqlgen from the config's directory |
| Tidy / verify modules | `make deps` | `go mod tidy` |
| Vulnerability scan | `make deps-audit` | `govulncheck ./...` |
| Start dependencies (Postgres) | `make compose-up` | `docker compose up` |
| Start the whole stack including the app | `make compose-up-all` | `docker compose up` |
| Tear down | `make compose-down` | `docker compose down` |
| Run the app locally | `make run` | `go run ./cmd/app` — the target regenerates docs and builds with `-tags migrate` |
| Run the worker locally (projections and queue; the app needs it) | `make run-worker` | `go run ./cmd/worker` |
| Rebuild one projection's read model | `make projection-rebuild name=<projector>` | `go run ./cmd/worker -rebuild ...` |
| Run everything in dev mode (tmux, hot reload) | `make dev` / `make dev-down` | `go tool air ...` in separate terminals — `scripts/dev.sh` passes the loaded env into every tmux pane and turns tracing off |
| Apply schema changes | `make schema-apply` | `atlas schema apply ...` |
| Add a domain (CRUD stubs on every layer and transport) | `make new-domain name=<name>` (`dry-run=1` first) | copying the `task` domain — the target checks every rule before it writes, then runs the regenerators and prints the undo; see `docs/generator.md` |
| Add a command or query to a domain | `make new-usecase domain=<domain> name=<name> kind=command\|query` (`dry-run=1` first) | writing the message, its registration and each transport's operation by hand, or `go -C tools/gen run .` — the target runs the regenerators after the write |
| Check the generator (lint, format, unit and golden tests, tidy) | `make gen-check` | `go test` in `tools/gen` — the target uses the root `.golangci.yml`, `-race` and `go mod tidy -diff` |
| Prove what the generator writes still builds, lints and tests | `make gen-e2e` | generating into your checkout — the target works in a throwaway worktree of the working tree and removes it |
| Full check before pushing | `make pre-commit` | running the steps by hand |
| See what master would be released as | `make release-version` (svu); the notes: `gh workflow run release.yml` (GoReleaser dry run) | tagging by hand — `release.yml` tags and releases on its own; see `docs/releasing.md` |

`make help` lists every target.

Traps in these targets:

- **`make integration-test` is not the one you want.** It runs `go test ./integration-test/...` on
  the host, where the suite cannot resolve the container hostnames it needs, so it always fails.
  `make compose-up-integration-test` is the real entry point.
- **`make run` and `make pre-commit` depend on `proto-v1`**, so they need `protoc` on `PATH`.
  Run `make bin-deps` first (`protoc` itself is not installed by it).
- **`make pre-commit` also runs the frontend targets**, so it needs the `vp` CLI (Vite+) on `PATH`;
  `vp` provisions the Node version in `web/.node-version` and bun itself.
- **`make new-domain`, `make new-usecase` and `make gen-e2e` run the regenerators** (`proto-v1`,
  `graphql`, `sqlc`, `mock`, `openapi`, `web-api`, `web-graphql`, `format`), so they need `protoc`,
  Docker, Atlas, `vp` and `make web-deps`, as those do. The generator refuses to edit a file with
  uncommitted changes (rule R3): commit first.

Never claim a change is done without `make format`, `make linter-golangci` and `make test` passing.
For changes under `web/`, or to any Huma input/output struct, `make web-api` and `make web-check` too.
For changes to a GraphQL schema file or to an operation in `web/src/`, `make web-graphql` too.
For changes under `tools/gen`, `make gen-check` too; for those, and for changes to a file a generator
template mirrors (see **Keep the docs current**), `make gen-e2e` too.
Commit subjects are Conventional Commits, and they name the release: a `feat`, `fix` or
breaking (`!`) commit merged to master is released as the next version (`docs/releasing.md`);
`perf`, `docs`, `ci`, `chore`, `test`, `build` and `refactor` release nothing. Never move or delete a `v*` tag.

## Dependency rule

```
cmd/app → internal/app → internal/controller/*  ── sends commands and queries on pkg/cqrs ──┐
                       → internal/usecase/<domain> (handlers, registered on the buses) ◀───┘
                       → internal/repo/*        ── implements internal/repo/contracts.go
internal/usecase/<domain> → internal/repo/contracts.go (interfaces) → internal/entity
```

- `internal/entity` — domain types (the read shapes the API returns), aggregates and their events,
  claims, and sentinel errors. Imports nothing from this module.
- `internal/usecase/<domain>/` — `command/`, `query/` and `event/`: one file per message holding the
  message type and its handler, plus `register.go`, which registers them on the buses. `aggregate/`
  (load, change, save) and `behavior/` (the app's pipeline steps) are shared by every domain. There
  are no use-case interfaces: a controller builds its domain's command or query and sends it.
- `internal/repo/contracts.go` — the interfaces handlers call: `EventStore`, one read model per
  domain, `Subscriber`, `QueueRepo` and the outbound ports. Implemented by
  `internal/repo/persistent/eventstore` (the event store), `persistent/projection` (the runner the
  worker runs), `persistent/<domain>/` (each domain's read model, projector and importer), and
  `internal/repo/webapi/` (outbound HTTP: the ExchangeRate-API rate client, plain `net/http`,
  unit-tested against the recorded response in `testdata/` that the integration stub also serves).
  `internal/repo/memstore` is the in-memory event store command-handler tests use; it and the
  Postgres store share `internal/repo/eventcodec` (codec, snapshot rule, claims, metadata).
- `internal/controller/<transport>/v1/<domain>/` — one folder per domain in each transport, holding
  that domain's handlers, DTOs and tests (GraphQL has no `v1/`). A controller takes `cqrs.Buses` and
  imports its domain's `command`/`query` packages only for the message types. Controllers never
  import each other; see **Controller layout** below.
- `pkg/` — transport-agnostic infrastructure (buses, servers, jwt, postgres, tracing). Logging is
  `log/slog`, built once in `internal/app` and passed as a `*slog.Logger`. Must
  not import `internal/`.
- `tools/gen` — the domain and use-case generator (`make new-domain`, `make new-usecase`), its own Go
  module (`github.com/m7medvision/go-clean-template/tools/gen`). It imports nothing from the app, and
  nothing in the app imports it: `go build ./...`, `make test`, the binaries and the image never
  include it, so `make gen-check` is what lints and tests it. See `docs/generator.md`.

Inner layers never import outer ones. Wiring happens exactly once, in `internal/app`: `buses.go`
(`newBuses`, `registerHandlers`, `projectors`, `eventHandlers`), `backfill.go` (`importers`),
`bootstrap.go` (what both binaries start from) and `app.go` / `worker.go` (`Run`, `RunWorker`).
Every long-running part (the HTTP and gRPC servers, the queue worker, the projection runner) is a
`Run(ctx) error` under one errgroup, on the context `signal.NotifyContext` gives each `main`;
nothing calls `Fatal` or `os.Exit` below `main`.

## Where code goes

Decide by asking what the code knows about:

| The code knows about… | It belongs in | Shape |
| --- | --- | --- |
| nothing but the domain | `internal/entity/<name>.go` | struct + methods (`TaskStatus.CanMoveTo`, `entity.NewConversion`) |
| a state change and its rules | `internal/entity/<name>_aggregate.go`, events in `<name>_events.go` | aggregate method that checks the rule and calls `entity.Record`; every event type listed in `EventTypes()` |
| one change a caller asks for | `internal/usecase/<domain>/command/<name>.go` | command type + handler returning `entity.Written`, one line in `register.go` |
| one thing a caller reads | `internal/usecase/<domain>/query/<name>.go` | query type + handler reading a read model, one line in `register.go` |
| a side effect that follows an event | `internal/usecase/<domain>/event/<name>.go` | `repo.Subscriber` (`Name`, `Handles`, `Handle`), listed in `eventHandlers` (`internal/app/buses.go`) |
| a concern every message shares (auth, retry, logging) | `internal/usecase/behavior` (app rules) or `pkg/cqrs` (generic) | a `cqrs.Step`, added in `newBuses` |
| SQL text | `internal/repo/persistent/queries/<domain>.sql` | an sqlc-annotated query, then `make sqlc` |
| a read-model table, a driver error code | `internal/repo/persistent/<domain>/<domain>.go` | read-model method calling the generated query |
| how events change a read-model table | `internal/repo/persistent/<domain>/projector.go` | one case per event its `Handles` lists, listed in `projectors` |
| rows a domain wrote before it stored events | `internal/repo/persistent/<domain>/importer.go` | `*Imported` events, listed in `importers` (`internal/app/backfill.go`) |
| an outbound HTTP API | `internal/repo/webapi/<name>.go` | method on the webapi struct |
| a transport status or error code, a message envelope | `internal/controller/<transport>/v1/<domain>/<domain>.go` (GraphQL: `graphql/<domain>/query.go`, `mutation.go`) | handler |
| a queue message (decode, send a command, ack or retry) | `internal/controller/worker/v1/<name>.go` | `HandlerFunc`, registered in `v1/router.go` |
| the data an email shows | `internal/entity/email_<name>.go` | struct with `Template()`/`Recipient()`, plus an entry in `EmailTypes()` (`email.go`) |
| an email's wording and markup | `internal/repo/mailtmpl/templates/<name>.{html,txt}.tmpl` | `content`, and `subject` + `text`; fixture and golden in `mailtmpl_test.go` |
| a server, pool, client or middleware with no domain knowledge | `pkg/<name>/` | reusable package |

Layout rules:

- In `usecase/<domain>/`, one file per message in `command/`, `query/` or `event/`, named after it
  in snake case (`create_task.go`), with its test beside it. Elsewhere, one file per domain per
  layer, named after the domain (`task.go`, `user.go`, `exchange.go`).
- Package name matches the directory name. A repository package shares its name with the use-case
  package of the same domain, so `internal/app` imports the repositories under aliases
  (`persistTaskRepo`, …).
- Repository constructors take their dependencies as arguments. `New` and `NewReadModel` return the
  *interface* from `contracts.go`; `NewProjector` and `NewImporter`
  return their struct, which `internal/app` lists as a `repo.Subscriber` or `repo.Importer`. A handler is a struct
  whose exported fields are its dependencies, filled in by `register.go`. No package-level state —
  `gochecknoglobals` and `gochecknoinits` are on.
- Unexported package constants are prefixed with an underscore (`_defaultEntityCap`,
  `_tracerName`). `mnd` rejects bare numeric literals, so add a constant rather than inlining one.

## Controller layout

Every transport splits by domain, then by job: **folders by domain, files by job, tests beside the
code.** The version comes first, so a v2 is one sibling folder (`restapi/v2/`) with the same
domain folders inside.

```
restapi/v1/<domain>/   controller.go (struct, NewRoutes), <domain>.go (Huma ops and their
                       input/output structs), request.go, response.go, <domain>_test.go,
                       fake_test.go
grpc/v1/<domain>/      controller.go (struct, New returning the proto server interface),
                       <domain>.go (RPCs), request.go, response.go, tests
graphql/<domain>/      resolver.go (Query/Mutation, New), query.go, mutation.go, input.go or
                       response.go, tests
```

- **A domain folder never imports another domain folder.** What several domains share has its own
  package in that transport: REST `v1/response/` (`Token`) and `v1/apidoc/` (the `BearerAuth`
  scheme, the `auth` tag); GraphQL `middleware/` and `gqlerr/`.
- Each version's `router.go` (`restapi/v1`, `grpc/v1`) is the only file that lists the domains: one
  call per domain, each passed the `cqrs.Buses`. `restapi/router.go`, `grpc/router.go` and `docs/api.md`'s v2 examples call it.
- Tests go through the real server and real buses, with fake handlers registered for the domain's
  messages (`fake_test.go`), from the domain folder: REST with `restapi/resttest` (it sets no env:
  no controller reads config), GraphQL with `graphql/graphqltest`, gRPC over
  `bufconn` in the test itself. Router-wide tests (public routes,
  auth exemption, panics, the OpenAPI export) stay in `restapi/router_test.go`.
- **A REST DTO's Go type name is its OpenAPI schema name** (Huma names schemas after the type), so
  renaming one changes `docs/openapi/v1.json` and the web client's types. A name that repeats its
  folder (`task.TaskList`) carries a `//nolint:revive` saying so.
- GraphQL: `resolver.go` at the root embeds each domain's `Query` and `Mutation` through a type
  alias (`taskQuery = task.Query`): Go names an embedded field after its type, so two domains'
  `Query` types would clash without one. gqlgen writes no resolver code (`gqlgen.yml` has no
  `resolver:` section), so it never rewrites a hand-written file; `generated/` has one file per
  schema file. GraphQL has no `v1/`: a schema evolves by adding fields and marking old ones
  `@deprecated`.

## The handler contract

Every controller method, on every transport, does the same six things in the same order. Copy the
shape from a neighbouring handler in the same transport.

1. **Get the caller.** REST: `middleware.UserID(ctx)` (the chi `middleware.Auth` middleware stores
   it in the request context).
   gRPC: `grpcmw.UserIDFromContext(ctx)`.
   GraphQL: `middleware.Caller(ctx)` (`graphql/middleware`).
   A failure here is an auth error, not a 500.
2. **Decode into a transport-local DTO** from the domain folder's `request.go`. On REST the DTO is
   the `Body` field of a small Huma input struct; never decode into an `entity` type, and never pass
   a request DTO into a command or query.
3. **Leave the rules to the domain.** Every input rule (lengths, formats, statuses, paging) is a
   message's `Validate()`, built from an `entity` function or constant (`entity.ValidateTaskText`,
   `entity.ValidateSignUp`, `entity.Page`), and the buses' validate step runs it for every
   transport. REST still writes the same limits as Huma tags, because they are the API's
   documentation and reject a bad request before it is decoded; `restapi/router_test.go` pins those
   tags to the entity constants. gRPC and GraphQL check nothing of their own. A rule about the
   record's state, like a status transition, lives on the aggregate.
4. **Send the command or query** with the request context:
   `cqrs.SendCommand(ctx, r.cmds, command.CreateTask{...})` or
   `cqrs.SendQuery(ctx, r.qs, query.GetTask{...})` (the message's embedded `cqrs.Command[R]` or
   `cqrs.Query[R]` gives the result type), with the Huma handler's `ctx` on REST
   (do not build a second context) and the handler's `ctx` everywhere else. Build the message from the
   DTO's fields; never put a DTO in it. A command answers only `entity.Written` (or a result embedding
   it, such as a sign-in's token): a transport that returns the resource sends a query for it next.
   `context.Context` is the first parameter of every method that crosses a layer.
5. **Map the error** with the transport's one function: `apierr.Problem`, `rpcerr.Status` or
   `gqlerr.Map`. Each `entity.Err*` is an `*entity.Error` carrying its kind (not found, forbidden,
   invalid, exists, conflict, unavailable, upstream, not implemented) and the message the caller is
   shown; the function turns the kind into the transport's status (on REST a `huma.Error*`, which
   renders the standard RFC 9457 `application/problem+json` model) and logs what needs logging.
   Any other error is logged and answered as a generic 500 / `Internal` / `INTERNAL`. A domain folder
   never maps errors itself: a new sentinel needs only its kind. A REST operation lists its statuses
   with `apierr.CommandErrors(...)` or `apierr.QueryErrors(...)`, which add the ones every command or
   query can answer.
6. **Encode the response.** Return an `entity` type directly only when its JSON shape is already
   the API shape — `entity.User` is safe this way because `PasswordHash` is tagged `json:"-"`.
   Anything with a wrapper, a projection or a different field set gets a type in the domain
   folder's `response.go` used as the Huma output `Body` (`TaskList`), or in `restapi/v1/response/`
   when several domains return it (`response.Token`).
   gRPC always converts through its folder's `New*Response`.

## Conventions that will bite you

**Pipeline steps, not decorators.** Every message passes the buses' steps, built in `newBuses`
(`internal/app/buses.go`): trace, log, metrics and stamp on both buses; wait (for the projections)
and retry (on `entity.ErrConcurrencyConflict`) on commands only; then authorize (`AdminOnly()`)
and validate (`Validate()`). A handler carries none of these. A message type with no registered
handler is `cqrs.ErrNoHandler` at the first send, so a new message needs its line in `register.go`.
Below the buses, otelpgx traces every SQL statement and otelhttp every outbound call, so
repositories carry no tracing code: a span per message, then one per statement or request.

**Events are forever.** A stored event is never rewritten. Changing an event's shape means raising
its `SchemaVersion()` and adding an `entity.Upcast` from the previous version to
`entity.Upcasts()`. A new event type needs an entry in `entity.EventTypes()`, a case in the
aggregate's `Apply`, and a case in the projector; an event the codec does not know fails to load.

**Writes go through aggregates; read-model tables belong to their projector.** A command handler
loads an aggregate, calls its behavior method and saves what it recorded (`aggregate.Save`,
`aggregate.Change`). A query reads only a read model and never loads an aggregate. Only a projector
writes its tables, so read-model tables have no foreign keys. Uniqueness across records (emails,
usernames, SSO identities) is a claim saved with the events, not a unique index on a read model.

**Mocks are generated, not written.** `internal/repo/repomock/repomock.go` comes from the
`go:generate` directive in `internal/repo/contracts.go`. After any repository contract change run
`make mock` before running tests. Query handlers are tested with these mocks; command handlers
against `internal/repo/memstore`; projectors against a fake of sqlc's `Querier`.

**Error style.** Domain failures are sentinel errors in `internal/entity/errors.go`
(`ErrTaskNotFound`, `ErrTaskForbidden`, `ErrInvalidTransition`, …), each an `*entity.Error` built
with its kind and the message callers see. Every other error is wrapped with the layer path:
`fmt.Errorf("CreateTask - aggregate.Save: %w", err)`, and is internal: logged, never shown. The
`err113` linter rejects `errors.New`/`fmt.Errorf` for new dynamic errors — add a sentinel to `entity`
instead. Only the transports' one mapping function each turns a kind into a transport status.

**Money is `decimal.Decimal`, never `float64`.** `entity.Conversion` holds `shopspring/decimal`
values; sqlc maps every `numeric` column to it (`sqlc.yaml` override) and pgx reads and writes it
through `sql.Scanner`/`driver.Valuer`. Transports carry decimals as strings: Huma cannot build a
schema for `decimal.Decimal`, so REST returns the `Conversion` DTO (`restapi/v1/exchange/response.go`),
not the entity.

**Config is environment-only.** `config/config.go` parses env vars with `caarlos0/env` into
exported sections (`config.OIDC`, `config.Worker`, …). Pass a part the section or the values it
reads, never `*config.Config`; no controller imports `config`. Defaults are written only in
`config/`: a `pkg/` constructor takes explicit values (a `pkg` type with a section's fields converts
from it, as `worker.Settings(cfg.Worker)`). A new
setting needs the struct field, an entry in `.env.example`, and — if the container needs it — the
`environment` block in `docker-compose.yml`. Fields tagged `required` make the app fail at
startup. `.env.example` is read both by `make` (`include`) and by the shell in CI
(`. ./.env.example`), so values there must not need quoting: no spaces.
`MAIL_FROM` is a bare address for that reason.

**The linter is strict and non-obvious.** `.golangci.yml` enables ~45 linters. The ones that
reject otherwise-fine code: `wsl_v5` and `nlreturn` (blank line required before `return` and
around blocks), `funlen` (65 lines / 40 statements), `gocyclo` (10), `gocognit` (15), `mnd`,
`gochecknoglobals`, `exhaustive` (switches over enum types need every case or a `default`), `dupl`
(threshold 100 — copy-pasting a handler across transports trips it).

## Generated files — do not hand-edit

- `docs/proto/v1/*.pb.go` ← `make proto-v1` from the `.proto` files beside them.
- `internal/repo/repomock/repomock.go` ← `make mock`.
- `internal/repo/persistent/queries/schema/schema.sql` ← `make sqlc`, rendered by Atlas from
  `internal/app/schema/*.hcl`. It exists only because sqlc cannot read HCL; CI fails when it is stale.
- `internal/repo/persistent/queries/*.go` ← `make sqlc` from the `.sql` query files beside them,
  that rendered schema, and `sqlc.yaml`.
- `internal/controller/graphql/generated/` and `internal/controller/graphql/model/` ← `make graphql`
  from `internal/controller/graphql/schema/*.graphqls` and `gqlgen.yml`. gqlgen writes no resolver
  code: the resolvers in `internal/controller/graphql/<domain>/` are hand-written, and no tool
  touches them.
- `web/src/api/schema.gen.ts` ← `make web-api` (openapi-typescript) from `docs/openapi/v1.json`.
- `web/src/graphql/` ← `make web-graphql` (GraphQL Code Generator, `web/codegen.ts`) from
  `internal/controller/graphql/schema/*.graphqls` and the `graphql(...)` operations in `web/src/`.
- `web/src/routeTree.gen.ts` ← the TanStack Router Vite plugin, on every `vp dev` / `vp build`.
- `docs/openapi/v1.json` ← `make openapi`, exported by `cmd/openapi` from the same Huma config the
  server uses (`restapi.OpenAPI`). Huma still serves the live document at `/openapi.json` when
  `DOCS_ENABLED`; the checked-in copy exists so REST contract changes show up as a reviewable diff
  and so clients can generate types without a running server. CI fails when it is stale.

## CQRS and event sourcing

`docs/cqrs.md` is the reference; these are the parts that break things when missed.

- **The event store** (`internal/repo/persistent/eventstore`) is the `events`, `snapshots` and
  `claims` tables. `Save` takes the store's advisory lock, so positions commit in order; checks the
  stream's version (`entity.ErrConcurrencyConflict` on a mismatch, which the retry step retries);
  inserts claims and events; then `NOTIFY events`. Keep that lock: without it a projector can pass a
  position whose earlier neighbour commits later, and never see it.
- **Projections run in the worker** (`internal/repo/persistent/projection`): one loop per
  subscriber (`projectors` and `eventHandlers`), each event of a type it `Handles` handled in one
  transaction with its checkpoint's move.
  A subscriber writes through `pg.Executor(ctx)` (`pkg/postgres`) so its rows join that
  transaction; anything else (a second pool, a goroutine) breaks exactly-once.
- **Every command waits for the projectors** (the wait step) until they pass its position, or
  answers `entity.ErrReadModelBehind` after `CQRS_CONSISTENCY_TIMEOUT`. Event handlers are not waited
  for. So the app and the integration tests need the worker running; `make dev` and the compose stack
  start it.
- **The backfill** (`eventstore.Backfill`, `internal/app/backfill.go`) runs at app start, once per
  domain, and turns rows written before event sourcing into `*Imported` events. Projectors apply
  `*Imported` as upserts. A new domain needs no importer: it never had rows without events.
- **Scratch streams** start with `_` and are never delivered; the event store's integration tests use
  only those, so they can share the database with a running worker. Subscriber names cannot start
  with `_` either: `_backfill/<domain>` markers live in `projection_checkpoints`.
- **Rebuild** a projector with `make projection-rebuild name=<projector>`, never by hand-editing a
  checkpoint. Event handlers cannot be rebuilt: replaying them repeats their side effects.

## Queue and worker

- The queue is one ordinary table, `queue_messages` (`internal/app/schema/queue_messages.hcl`), on the
  stock `postgres:18.6-alpine` image: no extension, no custom image. Its queries are in
  `internal/repo/persistent/queries/queue.sql` (sqlc), and `internal/repo/persistent/queue` implements
  `repo.QueueRepo` with them.
- `ReadQueueMessages` claims and hides in one statement (`FOR UPDATE SKIP LOCKED` in a CTE, then
  `UPDATE … SET visible_at = now() + vt`). Keep it one statement; splitting it lets two workers take
  the same message.
- `worker.Message` lives in `pkg/worker`, and `repo.QueueRepo` returns it, so the repo satisfies
  `worker.Queue` with no adapter (`var _ worker.Queue` in `queue.go`). The sqlc row converts with
  `worker.Message(row)`; a column change that breaks the field order fails to compile.
- `pkg/worker` is at-least-once: a handler returning `nil` deletes the message; an error leaves it
  to reappear after `WORKER_VISIBILITY_TIMEOUT`. Handlers must be idempotent; a body that can never
  decode should be logged and acknowledged (`return nil`), or it redelivers forever.
- The worker is wired in `internal/app/worker.go` (`RunWorker`): the projection runner and the queue
  consumer. Queue handlers send commands on the buses `registerHandlers` fills, like the servers. The
  Dockerfile builds `/app` and `/worker` into one image; the compose `worker` service runs
  `/worker`. Only the app applies the schema.
- A command or event handler enqueues by taking `repo.QueueRepo` and calling `Send`, which joins the
  transaction in `ctx`: from an event handler, the message commits with the checkpoint.
- A handler can also enqueue through a port that hides the queue: the `send-welcome-email` event
  handler (`internal/usecase/user/event`) calls `repo.UserNotifier.Welcome`, implemented by
  `internal/repo/notify`.
- Email goes through the `mail` queue (`entity.EmailQueue`): the sender writes the body with
  `entity.EncodeEmail`, and `controller/worker/v1/mail.go` reads it with `entity.DecodeEmail`. That
  handler is the one exception to "decode into a transport-local DTO" — both ends are this codebase.
  It sends the `DeliverEmail` command (`internal/usecase/mail`), whose handler renders with
  `internal/repo/mailtmpl` and sends through `pkg/smtp`; only the worker sends it.
  `registerHandlers` registers it for both binaries, and fails if a template is broken.
- Mailpit (compose service `mailpit`, UI and API on :8025) catches email in development and in the
  integration stack.
  `integration-test/mail_test.go` asserts on what reaches it.

## Frontend

`web/` is a client-rendered React app: TanStack Router (file-based routes in `web/src/routes/`) and
TanStack Query, built and checked with Vite+ (`vp`: Vite, Oxlint, Oxfmt, Vitest) and bun.

- It talks to the backend over REST and GraphQL, both on the same origin (the Vite proxy in dev),
  so there is no CORS. It imports nothing from Go, and nothing in Go imports or embeds it; `go.mod`
  has `ignore ./web` and `make format` skips it, because npm packages can ship `.go` files.
- Types are generated, never hand-written. REST: `$api` (openapi-react-query) or `api`
  (openapi-fetch) from `web/src/api/client.ts`, typed from `docs/openapi/v1.json`. GraphQL: write
  the operation with `graphql(...)` from `#/graphql`, run `make web-graphql`, and call it through
  `web/src/api/graphql.ts`. A Huma or schema change that breaks the client fails the type check in
  the same PR.
- **Tables read through GraphQL.** Every list shown in a table is a GraphQL query that selects only
  the columns the table shows, and the row actions on that screen use GraphQL mutations. A domain
  without a GraphQL schema needs one before it gets a table (tasks and exchange have one today). Forms and
  single-record screens outside a table (auth, profile) stay on REST.
- **Every table is a shadcn data table on TanStack Table (v9).** Build it with `useDataTable` and
  `dataTableColumns<T>()` from `web/src/components/data-table/`, render it with `DataTable`, and put
  `DataTableSearch`, `DataTableViewOptions` and `DataTablePagination` around it. Columns live in
  `web/src/<domain>/columns.tsx`, with a `meta.label` each and `DataTableColumnHeader` as the
  header. Paging is server-side (`limit`/`offset`, page number in the URL); sorting and search
  apply to the current page. Never hand-roll a `<Table>` with its own sort or visibility state.
- **Every form is TanStack Form + shadcn Field + Zod**, built with `useAppForm` from
  `web/src/components/form/use-app-form.ts`:
  - The rules live in a Zod schema in `web/src/<domain>/schema.ts` that mirrors the server's
    constraints (Huma tags for REST, the resolver's checks for GraphQL). Spread
    `zodValidation(schema)` into `useAppForm`: it validates on submit, then on every change.
  - Render inside `<form.AppForm><form.Form>…</form.Form></form.AppForm>`. Each input is a
    `<form.AppField name>` rendering `field.TextField`, `field.PasswordField` or
    `field.TextareaField` (shadcn `Field` with label, description, `aria-invalid` and
    `FieldError`). A new kind of input gets a field component in `web/src/components/form/fields.tsx`
    and an entry in `createFormHook`, not inline markup.
  - `onSubmit` returns `mutation.mutateAsync(schema.parse(value))`, so the server gets the parsed
    (trimmed) value and `form.SubmitButton` stays disabled until the request ends. Show the
    server's rejection with `<FormError error={mutation.error} />`.
  - No `useState` per input, no `required`/`minLength` attributes (`Form` sets `noValidate`; Zod
    is the only client-side validator).
- **All server state goes through TanStack Query**, in one shape:
  - Each domain keeps its operations in `web/src/<domain>/api.ts`: query-option factories
    (`graphqlQueryOptions` / `$api.queryOptions`), mutation options (`graphqlMutationOptions` /
    `$api.useMutation`), and the key prefix its writes invalidate (`graphqlKey`).
  - Route loaders call `queryClient.ensureQueryData(...)`; components read with `useSuspenseQuery`
    on the same options, so the loader and the component share one cache entry.
  - Writes use `useMutation` and, `onSuccess`, `invalidateQueries` on the domain's key prefix.
    No `fetch` in components, no copying server data into `useState`.
  - Errors reach the UI through `problemMessage` (`web/src/api/problem.ts`), which reads both Huma
    problems and GraphQL errors. GraphQL reports an expired token as `UNAUTHENTICATED` in a 200
    response; `web/src/api/graphql.ts` turns that into the same sign-out a REST 401 triggers.
- The JWT lives in memory, mirrored to `sessionStorage` (`web/src/auth/token.ts`). Screens that need
  a user go under the `_authed` layout route.
- Client routes must not use the prefixes the Go app owns: `/v1`, `/graphql`, `/docs`,
  `/openapi.*`, `/schemas`, `/healthz`, `/metrics`.
- Tests import from `vite-plus/test` and sit next to the code as `*.test.ts`.

## Keep the docs current

`README.md` is a short front page for a visitor: what the template is, the Clean Architecture
rules in brief, a Quick start, and links. Reference detail lives in `docs/*.md`, never in the README; a
paragraph about one package, an env var table or an endpoint list in the README is a bug. Update
the matching page whenever a change touches what it documents:

| Change | What to update |
| --- | --- |
| New or changed operation on any transport | the domain table in `docs/api.md` |
| New domain | a section in `docs/api.md`, the `What you get` list in `README.md` |
| New or renamed env var | the matching table in `docs/configuration.md` |
| New transport, server, port or service URL | `docs/development.md` (service tables), `docs/api.md` if it is an API |
| Change to `make dev` or its prerequisites | `Quick start` in `README.md`, `docs/development.md` |
| Renamed or moved package under `internal/` or `pkg/` | the `Packages` table in `docs/architecture.md` |
| Commands, queries, events, the event store, projections or the pipeline steps | `docs/cqrs.md` |
| Queue or worker behaviour | `docs/worker.md` |
| Email behaviour | the `Email` section of `docs/worker.md` |
| Frontend stack or conventions | `docs/web.md` |
| Release rules, `release.yml` or `.goreleaser.yaml` | `docs/releasing.md` |
| The exchange `Convert` path (handler, `ConvertCurrency` command, aggregate, webapi, event store, projector, query) | `docs/request-flow.md`, which quotes that code |
| A layering rule (what may import what, where code goes) | `Rules` in `docs/architecture.md`, and the `Clean Architecture` section of `README.md` |
| A file a generator template mirrors: a `task` domain file (each template under `tools/gen/templates` copies the `task` file at its path), or a shared file the generator edits (`entity/event.go`, `entity/errors.go`, `repo/contracts.go`, `app/buses.go`, the REST and gRPC `v1/router.go`, `graphql/resolver.go`, `gqlerr/gqlerr.go`, `gqlgen.yml`; for `new-usecase`, a domain's `register.go`, `.proto`, schema and transport files) | its template under `tools/gen/templates` and its golden (`UPDATE_GOLDEN=1 go -C tools/gen test ./...`, then review the diff); `make gen-check` and `make gen-e2e` must pass |
| The generator's commands, flags, rules, output or template conventions | `docs/generator.md` |

## Adding a use case to an existing domain

A use case is a command (it changes state) or a query (it reads it). Start with the generator
(`docs/generator.md`), then read the neighbouring use case in the same domain before writing its
body:

```sh
make new-usecase domain=task name=archive-task kind=command dry-run=1  # the plan; writes nothing
make new-usecase domain=task name=archive-task kind=command            # or kind=query
```

It writes the message type and a stub handler returning `entity.ErrNotImplemented`, with a test that
asserts so (steps 1 and 2); its line in `register.go` (step 3); and, on each of the domain's
transports, its operation and a test (step 5): the REST op and its registration, the rpc with its
messages and the gRPC method, the GraphQL field and its resolver. The REST route defaults to
`POST /v1/<plural>/{id}/<verb>` for a command and `GET /v1/<plural>/<verb>` for a query; `method=`,
`path=` and `transports=` change the defaults. Then it runs the regenerators. What it leaves to you:

- the handler's body, and tests of what it does in place of the stub's test;
- for a command, the event it records, the aggregate's behavior method with its `Apply` case, and the
  projector's case; `AdminOnly()` or `Validate()` if it needs them;
- for a query, the read-model method with its SQL, `make sqlc` and `make mock`, and the fields of its
  result on each transport;
- its row in `docs/api.md`.

The steps below are what the generated files hold, and the recipe by hand when a rule refuses the
generator. Its transport tests are self-contained, a fake on buses of their own, so it leaves
`fake_test.go` alone.

1. **A command:**
   - If it records something new, add the event to `internal/entity/<domain>_events.go`
     (`EventType()`, `SchemaVersion()`), list it in `EventTypes()`, give the aggregate in
     `<domain>_aggregate.go` a behavior method that checks the rule and calls `entity.Record`, and
     a case in its `Apply`. Test it in `internal/entity`.
   - Add `internal/usecase/<domain>/command/<name>.go`: the command type (embedding
     `cqrs.Command[entity.Written]`, with `ActorID()` when it acts for a user, `AdminOnly()` when only an admin may send it, `Validate()` for
     domain checks) and its handler, which goes through `aggregate.Save` or `aggregate.Change` and
     returns `entity.Written`. Test it beside it against `internal/repo/memstore`.
   - Handle the new event in the domain's projector (`internal/repo/persistent/<domain>/projector.go`),
     with any new SQL in `queries/<domain>.sql` and `make sqlc`; test it against the fake `Querier`.
2. **A query:** add `internal/usecase/<domain>/query/<name>.go`: the query type (embedding
   `cqrs.Query[R]`, R being what it answers) and
   its handler, which reads only the domain's read model. A new read-model method goes in
   `internal/repo/contracts.go` and `internal/repo/persistent/<domain>/<domain>.go`, with its SQL in
   `queries/<domain>.sql` and `make sqlc`; then `make mock`. Test the
   handler against `internal/repo/repomock`.
3. **Register it**: one line in `internal/usecase/<domain>/register.go`. A message with no handler is
   `cqrs.ErrNoHandler` at the first send.
4. Tests are table-driven where there are cases and call `t.Parallel()` (`paralleltest` enforces it).
5. Expose it on whichever transports need it, sending the message (see **The handler contract**):
   - REST: Huma input/output structs and the op in `internal/controller/restapi/v1/<domain>/<domain>.go`,
     DTOs in that folder's `request.go`/`response.go`, the op registered in its `controller.go`
     (`NewRoutes`), the fake handler in `fake_test.go` and the test beside it; then `make openapi`,
     and commit the generated diff.
     Then `make web-api`, commit its diff too, and fix any `make web-check` errors in `web/`.
   - GraphQL (task and exchange today): schema change in `internal/controller/graphql/schema/` →
     `make graphql` → the build fails with `missing method <Field>` → write that method in
     `internal/controller/graphql/<domain>/query.go` or `mutation.go`, with the signature from
     `generated.QueryResolver` / `generated.MutationResolver`, and its test beside it.
     Then `make web-graphql` and fix any `make web-check` errors in `web/`.
   - gRPC: `.proto` change → `make proto-v1` → controller method in
     `internal/controller/grpc/v1/<domain>/<domain>.go`, its mapping in that folder's `response.go`.

Transports are independent by design; adding a method to only one is a legitimate choice, not an
oversight — but say which ones you covered.

## Adding a whole domain

Start with the generator (`docs/generator.md`):

```sh
make new-domain name=invoice dry-run=1  # the plan; writes nothing
make new-domain name=invoice            # plural= when English gets it wrong, transports= for fewer than three
```

It writes steps 1 to 7 and 9 for a user-owned domain with create, update, delete, get and list on
the base fields (`id`, `user_id`, `created_at`, `updated_at`): 43 files on all three transports, and
the edits to the shared files those steps name. The command and query handlers are stubs returning
`entity.ErrNotImplemented`, so every operation answers 501 / `Unimplemented` / `NOT_IMPLEMENTED`;
everything they sit on (aggregate, events, read model, projector, wiring, transport handlers, tests)
is real. Then it runs the regenerators, `make openapi` among them. What it leaves to you:

- the five handlers' bodies, and tests of what they do in place of the stubs' tests;
- the domain's fields, where the generated comments say: the read shape, the events and `Apply`, the
  HCL table and its SQL, the projector, the request DTOs and messages, and the GraphQL type;
- the docs of step 8;
- writing this domain before generating another: two untouched generated domains are clones to
  `dupl`, and `make linter-golangci` fails.

Work outward from the entity. Every step has a counterpart in the existing `task` domain, which the
generated files copy — read it before writing. The steps are what the generated files hold.

1. **Entity** — `internal/entity/<domain>.go` with the read shape the API returns;
   `<domain>_aggregate.go` with the aggregate (embedding `entity.AggregateBase`, its stream ID
   `<domain>-<id>`, `Apply`, and the behavior methods that record events); `<domain>_events.go` with
   the events, added to `EventTypes()`. New sentinel errors go in `internal/entity/errors.go`. A
   value unique across records is a claim (`entity.Claim`), saved with the events.
2. **Schema** — the read-model table as `internal/app/schema/<domain>.hcl` (Atlas declarative
   workflow), with no foreign keys: its projector advances on its own. Then `make sqlc` to refresh
   the rendered schema sqlc reads; set any new env vars in `docker-compose.yml` and `.env.example`.
3. **Read-model contract** — add `<Domain>ReadModel` to `internal/repo/contracts.go`, then
   `make mock`.
4. **Read model and projector** — in `internal/repo/persistent/<domain>/`: `<domain>.go` (`package
   <domain>`, the read-model methods calling the sqlc queries from `queries/<domain>.sql`, driver
   errors mapped to sentinels, `NewReadModel` returning the interface), and `projector.go`
   (`NewProjector`, `Name`, `Tables`, one case per event
   in `Handle`, writing through `pg.Executor(ctx)`), each with its test.
5. **Commands, queries and event handlers** — `internal/usecase/<domain>/command/`, `query/` and,
   for side effects, `event/`, one file per message with its test (see the recipe above), and
   `register.go` with the domain's `Deps` and `Register`.
6. **Wiring** — in `internal/app/buses.go`: a `<domain>.Register(...)` call in `registerHandlers`,
   the projector in `projectors` (and any event handler in `eventHandlers`). The transports already receive the
   buses in `Run`.
7. **Transports** — for each one, a new domain folder (see **Controller layout**):
   `<transport>/v1/<domain>/` with its `controller.go`, handlers, `request.go`, `response.go` and
   tests (with a `fake_test.go` registering fake handlers), plus one call in that transport's
   `v1/router.go` that registers it.
   gRPC also needs `docs/proto/v1/<domain>.proto` and `make proto-v1`.
   GraphQL instead gets `schema/<domain>.graphqls`, a `graphql/<domain>/` folder, and an alias plus
   an embed for its `Query`/`Mutation` in `graphql/resolver.go`.
8. **Docs** — `make openapi` to refresh `docs/openapi/v1.json`, then the docs updates listed
   above, including the aggregate and projector tables in `docs/cqrs.md`.
9. **Integration test** — `integration-test/<domain>_test.go` exercising the domain over the
   transports you exposed, against the running worker.

## API versioning

Routes are grouped under a version package (`<transport>/v1/`). A v2 means a new sibling package
with the same layout, registered alongside v1 in the transport's top-level `router.go` — REST adds
operations under the new `/v2/...` paths from a `v2.NewRoutes(api, b, l)` in `restapi/router.go`.
gRPC calls a second `v2.NewRoutes(app, b, l)` in `grpc/router.go`. Both versions send the same
commands and queries.
v1 stays untouched. `docs/api.md` documents this
with worked examples per transport; follow it rather than inventing a scheme.

## Database

The database schema is managed with [Atlas](https://atlasgo.io/) using the declarative workflow:
the desired state lives in `internal/app/schema/*.hcl`, one file per table, embedded into the
binary; change the schema by editing the matching file, review the diff with `make schema-apply`
(host-side), and let Atlas apply it. The app applies the
schema itself at startup only when built with the `migrate` build tag (`internal/app/migrate.go`)
— that is what `make run`, `make dev` (`.air.app.toml`) and the Dockerfile do; startup apply needs only `PG_URL`.
There is no Atlas dev database: the stack runs exactly one database, and the apply diffs the HCL
straight against it. Do not add one back. The apply is declarative, so it **drops** tables and
columns the HCL does not declare. Make sure `PG_URL` points at this project's database before you
start the app. `PG_PORT` moves the compose `db` host port and `PG_URL` together. `ATLAS_DEV_URL` is
read only by `make sqlc`, where it is a throwaway `docker://` container. Queries are plain SQL
in `internal/repo/persistent/queries/*.sql`, compiled by [sqlc](https://sqlc.dev/) into typed Go
(`make sqlc`). sqlc cannot read HCL, so `make sqlc` first has Atlas render the HCL schema to
`internal/repo/persistent/queries/schema/schema.sql`; edit the HCL, never that file. The read models and projectors call the generated
`*queries.Queries` (projectors through the `Querier` interface, so they can be tested with a fake)
and convert its rows to `entity` types; there is no ORM and no query building at runtime. The event
store's own tables (`events`, `snapshots`, `claims`, `projection_checkpoints`) are HCL files like
any other. Generated row structs share the entities' field order, so `entity.Task(row)` is a
plain conversion. A schema change that breaks that alignment fails to compile.

## Tests

- Unit: `internal/...` and `pkg/...`, run by `make test` with `-race`, all calling `t.Parallel()`
  (`paralleltest` enforces it). Aggregates are tested in `internal/entity`; command handlers against
  the in-memory event store (`internal/repo/memstore`); query handlers with the gomock read models
  (`internal/repo/repomock`); projectors against a fake sqlc `Querier`. `memstore` and the Postgres
  event store pass the same contract suite, `internal/repo/eventstoretest`.
- Integration: `integration-test/` runs *inside* the docker network — it resolves the service as
  host `app` by its compose service name, so it fails on the host machine.
  Always run it through `make compose-up-integration-test`. It needs the worker: writes wait for
  the projections.
- Mail templates: `internal/repo/mailtmpl/testdata/*.golden.*` are compared by the render test;
  after an intended template change, `UPDATE_GOLDEN=1 go test ./internal/repo/mailtmpl/` and review the diff.
- Generator: `make test` does not reach `tools/gen`, its own module; `make gen-check` runs its unit
  and golden tests, and `make gen-e2e` generates a domain and three use cases in a throwaway worktree
  and lints and tests them. `tools/gen/testdata/golden/` holds what it creates for `widget`; after an
  intended template change, `UPDATE_GOLDEN=1 go -C tools/gen test ./...` and review the diff.
