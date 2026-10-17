# go-clean-template

Go clean-architecture reference service. Three domains (`user`, `task`, `translation`) exposed
over three transports (REST — chi + Huma v2, gRPC, and GraphQL via gqlgen at
`/graphql` on the REST port, tasks only, GraphiQL at `/graphql/playground` when `DOCS_ENABLED`) from one shared use-case layer.
Module path: `github.com/evrone/go-clean-template`. The Go version is declared in `go.mod`.
A React client (TanStack Router + Query, built with Vite+) lives in `web/`; see **Frontend** below.

## Commands — drive everything through the Makefile

The `Makefile` loads `.env` (falling back to `.env.example`) and exports it into every target, and
several targets carry flags that matter. Running the underlying tool directly means running it with
different settings than CI. Use the target, not the tool.

| Task | Use this | Not this |
| --- | --- | --- |
| Install tool binaries (`mockgen`, linters, `atlas`) | `make bin-deps` | `go install ...` |
| Unit tests | `make test` | `go test ./...` — the target adds `-race -covermode atomic` and scopes to `./internal/... ./pkg/...` |
| Integration tests | `make compose-up-integration-test` | `make integration-test` — see below |
| Lint | `make linter-golangci` | `golangci-lint run` |
| Format | `make format` | `gofmt` — the target runs `go fix`, `gofumpt`, and `gci` with the repo's import grouping |
| Regenerate mocks | `make mock` | `mockgen ...` |
| Regenerate protobuf | `make proto-v1` | `protoc ...` |
| Regenerate SQL query code | `make sqlc` | `sqlc generate` — the target first renders the HCL schema to SQL with Atlas (needs Docker for `ATLAS_DEV_URL`), then runs the sqlc pinned in `go.mod` |
| Regenerate the REST OpenAPI document | `make openapi` | `go run ./cmd/openapi` — the target writes through a temp file so a failed run never truncates `docs/openapi/v1.json` |
| Install frontend dependencies | `make web-deps` | `bun install` / `vp install` — the target uses the frozen lockfile |
| Regenerate frontend REST types | `make web-api` | `npx openapi-typescript ...` — the target refreshes `docs/openapi/v1.json` first |
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
| Apply schema changes | `make schema-apply` | `atlas schema apply ...` |
| Full check before pushing | `make pre-commit` | running the steps by hand |

`make help` lists every target.

Three traps in these targets:

- **`make integration-test` is not the one you want.** It runs `go test ./integration-test/...` on
  the host, where the suite cannot resolve the container hostnames it needs, so it always fails.
  `make compose-up-integration-test` is the real entry point.
- **`make run` and `make pre-commit` depend on `proto-v1`**, so they need `protoc` on `PATH`.
  Run `make bin-deps` first (`protoc` itself is not installed by it).
- **`make pre-commit` also runs the frontend targets**, so it needs the `vp` CLI (Vite+) on `PATH`;
  `vp` provisions the Node version in `web/.node-version` and bun itself.

Never claim a change is done without `make format`, `make linter-golangci` and `make test` passing.
For changes under `web/`, or to any Huma input/output struct, `make web-api` and `make web-check` too.

## Dependency rule

```
cmd/app → internal/app → internal/controller/*  ─┐
                       → internal/repo/*        ─┤→ internal/usecase (interfaces)
                                                 └→ internal/entity
```

- `internal/entity` — domain types and sentinel errors. Imports nothing from this module.
- `internal/usecase/contracts.go` — the interfaces controllers call. `internal/usecase/<domain>/`
  implements them.
- `internal/repo/contracts.go` — the interfaces use cases call. `internal/repo/persistent/<domain>/`
  (Postgres) and `internal/repo/webapi/` (outbound HTTP) implement them.
- `internal/controller/<transport>/v1/` — one package per transport, each with its own
  `request/` and `response/` DTOs. Controllers never import each other.
- `pkg/` — transport-agnostic infrastructure (servers, logger, jwt, postgres, tracing). Must not
  import `internal/`.

Inner layers never import outer ones. Wiring happens exactly once, in `internal/app/app.go`
(`initUseCases`, `initServers`).

## Where code goes

Decide by asking what the code knows about:

| The code knows about… | It belongs in | Shape |
| --- | --- | --- |
| nothing but the domain | `internal/entity/<name>.go` | struct + methods (`Task.Transition`, `TaskStatus.Valid`) |
| a domain rule that spans repositories | `internal/usecase/<domain>/<domain>.go` | method on `UseCase` |
| SQL text | `internal/repo/persistent/queries/<domain>.sql` | an sqlc-annotated query, then `make sqlc` |
| a table, a driver error code | `internal/repo/persistent/<domain>/<domain>.go` | method on `Repo` calling the generated query |
| an outbound HTTP API | `internal/repo/webapi/<name>.go` | method on the webapi struct |
| an HTTP status, a gRPC code, a message envelope | `internal/controller/<transport>/v1/<domain>.go` | handler |
| a server, pool, client or middleware with no domain knowledge | `pkg/<name>/` | reusable package |

Layout rules:

- One file per domain per layer, named after the domain (`task.go`, `user.go`, `translation.go`),
  plus a `tracing.go` in every `usecase/<domain>/` and `repo/persistent/<domain>/` package.
- Package name matches the directory name. `internal/repo/persistent/translation/` is the one
  violation in the tree — it declares `package persistent`, which is why `internal/app/app.go`
  imports the three repositories under aliases. Follow `task/` and `user/`, not `translation/`.
- Constructors are always `New`, return the *interface* from `contracts.go`, and take their
  dependencies as arguments. No package-level state — `gochecknoglobals` and `gochecknoinits` are on.
- Unexported package constants are prefixed with an underscore (`_defaultEntityCap`,
  `_tracerName`). `mnd` rejects bare numeric literals, so add a constant rather than inlining one.

## The handler contract

Every controller method, on every transport, does the same six things in the same order. Copy the
shape from a neighbouring handler in the same transport.

1. **Get the caller.** REST: `middleware.UserID(ctx)` (the chi `middleware.Auth` middleware stores
   it in the request context). gRPC: `grpcmw.UserIDFromContext(ctx)`. GraphQL: `caller(ctx)`.
   A failure here is an auth error, not a 500.
2. **Decode into a transport-local DTO** from `<transport>/v1/request/`. On REST the DTO is the
   `Body` field of a small Huma input struct; never decode into an `entity` type, and never pass a
   `request.*` type into a use case.
3. **Validate the DTO structurally through Huma's built-in validation** on the REST transport:
   length, format, enum, and default constraints live on the input struct / request DTO tags, and
   the REST v1 controller takes no `*validator.Validate` (other transports still construct their
   own). Domain validation — anything that needs to know the rules, like a status transition —
   lives on the entity or the use case and must not be duplicated in the controller.
4. **Call the use case** with the request context: the Huma handler's `ctx` on REST (do not build a
   second context), the handler's `ctx` everywhere else. `context.Context`
   is the first parameter of every method that crosses a layer.
5. **Map the error.** `errors.Is` against the `entity.Err*` sentinels → a transport status. On REST
   the sentinel maps to a `huma.Error*` constructor, which renders the standard RFC 9457
   `application/problem+json` model — no custom error type. Log the
   wrapped error, return a generic message to the caller.
6. **Encode the response.** Return an `entity` type directly only when its JSON shape is already
   the API shape — `entity.User` is safe this way because `PasswordHash` is tagged `json:"-"`.
   Anything with a wrapper, a projection or a different field set gets a type in
   `<transport>/v1/response/` used as the Huma output `Body` (`response.TaskList`,
   `response.Token`). gRPC always converts through `response.New*Response`.

## Conventions that will bite you

**Traced decorators.** Every repo and use-case constructor returns the *interface*, wrapped:
`func New(...) usecase.Task { return newTraced(&UseCase{...}) }`. The wrapper lives in
`tracing.go` next to each implementation. Adding a method to a contract means implementing it in
three places: the interface, the struct, and the `traced*` decorator. Skipping the decorator is a
compile error that reads as an unrelated interface mismatch.

**Mocks are generated, not written.** `internal/usecase/mocks_*_test.go` come from `go:generate`
directives in the two `contracts.go` files. After any contract change run `make mock` before
running tests.

**Error style.** Domain failures are sentinel errors in `internal/entity/errors.go`
(`ErrTaskNotFound`, `ErrTaskForbidden`, `ErrInvalidTransition`, …). Every other error is wrapped
with the layer path: `fmt.Errorf("TaskUseCase - Create - uc.repo.Store: %w", err)`. The `err113`
linter rejects `errors.New`/`fmt.Errorf` for new dynamic errors — add a sentinel to `entity`
instead. Controllers are the only place that translates a sentinel into a transport status.

**Config is environment-only.** `config/config.go` parses env vars with `caarlos0/env`. A new
setting needs the struct field, an entry in `.env.example`, and — if the container needs it — the
`environment` block in `docker-compose.yml`. Fields tagged `required` make the app fail at
startup.

**The linter is strict and non-obvious.** `.golangci.yml` enables ~45 linters. The ones that
reject otherwise-fine code: `wsl_v5` and `nlreturn` (blank line required before `return` and
around blocks), `funlen` (65 lines / 40 statements), `gocyclo` (10), `gocognit` (15), `mnd`,
`gochecknoglobals`, `exhaustive` (switches over enum types need every case or a `default`), `dupl`
(threshold 100 — copy-pasting a handler across transports trips it).

## Generated files — do not hand-edit

- `docs/proto/v1/*.pb.go` ← `make proto-v1` from the `.proto` files beside them.
- `internal/usecase/mocks_*_test.go` ← `make mock`.
- `internal/repo/persistent/queries/schema/schema.sql` ← `make sqlc`, rendered by Atlas from
  `internal/app/schema/*.hcl`. It exists only because sqlc cannot read HCL; CI fails when it is stale.
- `internal/repo/persistent/queries/*.go` ← `make sqlc` from the `.sql` query files beside them,
  that rendered schema, and `sqlc.yaml`.
- `internal/controller/graphql/generated/` and `internal/controller/graphql/model/` ← `make graphql`
  from `internal/controller/graphql/schema/*.graphqls` and `gqlgen.yml`. The resolver bodies in
  `internal/controller/graphql/task.resolvers.go` are hand-written, but gqlgen rewrites that file in
  place on every run (signatures follow the schema, bodies are kept).
- `web/src/api/schema.gen.ts` ← `make web-api` (openapi-typescript) from `docs/openapi/v1.json`.
- `web/src/routeTree.gen.ts` ← the TanStack Router Vite plugin, on every `vp dev` / `vp build`.
- `docs/openapi/v1.json` ← `make openapi`, exported by `cmd/openapi` from the same Huma config the
  server uses (`restapi.OpenAPI`). Huma still serves the live document at `/openapi.json` when
  `DOCS_ENABLED`; the checked-in copy exists so REST contract changes show up as a reviewable diff
  and so clients can generate types without a running server. CI fails when it is stale.

## Frontend

`web/` is a client-rendered React app: TanStack Router (file-based routes in `web/src/routes/`) and
TanStack Query, built and checked with Vite+ (`vp`: Vite, Oxlint, Oxfmt, Vitest) and bun.

- It talks to the backend only over REST, on the same origin (the Vite proxy in dev), so there is no
  CORS. It imports nothing from Go, and nothing in Go imports or embeds it; `go.mod` has
  `ignore ./web` and `make format` skips it, because npm packages can ship `.go` files.
- Types come from `docs/openapi/v1.json`, never hand-written: call the API through `$api`
  (openapi-react-query) or `api` (openapi-fetch) from `web/src/api/client.ts`. A Huma change that
  breaks the client fails the type check in the same PR.
- The JWT lives in memory, mirrored to `sessionStorage` (`web/src/auth/token.ts`). Screens that need
  a user go under the `_authed` layout route.
- Client routes must not use the prefixes the Go app owns: `/v1`, `/graphql`, `/docs`,
  `/openapi.*`, `/schemas`, `/healthz`, `/metrics`.
- Tests import from `vite-plus/test` and sit next to the code as `*.test.ts`.

## Keep README.md current

`README.md` is the only README. The Russian and Chinese translations were removed, so there is no
set to keep aligned. Update it whenever a change touches what it documents:

| Change | What to update in `README.md` |
| --- | --- |
| New or changed REST / gRPC endpoint | the domain table under `Domains` |
| New domain | the `Overview` bullet list, `Content` index, and a new `Domains` subsection |
| New or renamed env var | the config table under `Observability` if tracing-related; otherwise the `config` section |
| New transport, server, port or service URL | the `Quick start` service list |
| New `make` target used in the getting-started flow | the `Quick start` code blocks |
| Renamed or moved package under `internal/` or `pkg/` | the matching `Project structure` subsection |

## Adding a use case to an existing domain

1. Add the method to `internal/usecase/contracts.go`.
2. Implement it in `internal/usecase/<domain>/<domain>.go` and mirror it in the same package's
   `tracing.go`.
3. If it needs new persistence, repeat the same three steps in `internal/repo/contracts.go`,
   `internal/repo/persistent/<domain>/`, and that package's `tracing.go`. A new query goes in
   `internal/repo/persistent/queries/<domain>.sql`, then `make sqlc`.
4. `make mock`, then extend `internal/usecase/<domain>_test.go` (table-driven, gomock, `t.Parallel()` —
   `paralleltest` enforces it).
5. Expose it on whichever transports need it:
   - REST: Huma input/output structs + registration in `internal/controller/restapi/v1/<domain>.go`
     (op registered in the same domain file), then `make web-api` (which runs `make openapi`), commit both
     generated diffs, and fix any `make web-check` errors in `web/`.
   - GraphQL (task only today): schema change in `internal/controller/graphql/schema/` →
     `make graphql` → fill the generated resolver stub in `<domain>.resolvers.go`.
   - gRPC: `.proto` change → `make proto-v1` → controller method in
     `internal/controller/grpc/v1/`.

Transports are independent by design; adding a method to only one is a legitimate choice, not an
oversight — but say which ones you covered.

## Adding a whole domain

Work outward from the entity. Every step has a counterpart in the existing `task` domain — read it
before writing.

1. **Entity** — `internal/entity/<domain>.go` with the type and its invariants; new sentinel errors
   go in `internal/entity/errors.go`.
2. **Schema** — add the table as `internal/app/schema/<domain>.hcl` (Atlas declarative workflow),
   then `make sqlc` to refresh the rendered schema sqlc reads;
   set any new env vars in `docker-compose.yml` and `.env.example`.
3. **Repo contract** — add the interface to `internal/repo/contracts.go`.
4. **Repo implementation** — `internal/repo/persistent/<domain>/<domain>.go` (`package <domain>`,
   calling the sqlc queries from `internal/repo/persistent/queries/<domain>.sql` after `make sqlc`,
   driver errors mapped to sentinels) plus `tracing.go` next to it, and a `New` returning the
   interface wrapped in `newTraced`.
5. **Use-case contract** — add the interface to `internal/usecase/contracts.go`.
6. **Use-case implementation** — `internal/usecase/<domain>/<domain>.go` plus its `tracing.go`,
   same `New` pattern.
7. **`make mock`**, then `internal/usecase/<domain>_test.go` against the generated mocks.
8. **Wiring** — add a field to `useCases` in `internal/app/app.go`, construct repo and use case in
   `initUseCases`, and thread the use case through `initServers` into each router you are exposing.
9. **Transports** — for each one: DTOs in `<transport>/v1/request/` and `response/`, handlers in
   `<transport>/v1/<domain>.go`, registration in that transport's `v1/router.go`, and the use case
   added to the `V1` / controller struct in `<transport>/v1/controller.go`. gRPC also needs
   `docs/proto/v1/<domain>.proto` and `make proto-v1`.
10. **Docs** — `make openapi` to refresh `docs/openapi/v1.json`, then the README updates listed
    above.
11. **Integration test** — `integration-test/<domain>_test.go` exercising the domain over the
    transports you exposed.

## API versioning

Routes are grouped under a version package (`<transport>/v1/`). A v2 means a new sibling package
with the same layout, registered alongside v1 in the transport's top-level `router.go` — REST adds
operations under the new `/v2/...` paths from a `v2.NewRoutes(api, ...)` in `restapi/router.go`,
and gRPC registers a second set of `NewXRoutes`. v1 stays untouched. `README.md` documents this
with worked examples per transport; follow it rather than inventing a scheme.

## Database

The database schema is managed with [Atlas](https://atlasgo.io/) using the declarative workflow:
the desired state lives in `internal/app/schema/*.hcl`, one file per table, embedded into the
binary; change the schema by editing the matching file, review the diff with `make schema-apply`
(host-side, uses Atlas's docker dev-database driver), and let Atlas apply it. The app applies the
schema itself at startup only when built with the `migrate` build tag (`internal/app/migrate.go`)
— that is what `make run` and the Dockerfile do; startup apply needs `PG_URL` and an empty,
dedicated `ATLAS_DEV_URL`.
The compose stack provides that dev database as the `atlas-dev` service. Queries are plain SQL
in `internal/repo/persistent/queries/*.sql`, compiled by [sqlc](https://sqlc.dev/) into typed Go
(`make sqlc`). sqlc cannot read HCL, so `make sqlc` first has Atlas render the HCL schema to
`internal/repo/persistent/queries/schema/schema.sql`; edit the HCL, never that file. The repositories call the generated
`*queries.Queries` and convert its rows to `entity` types; there is no ORM and no query building
at runtime. Generated row structs share the entities' field order, so `entity.Task(row)` is a
plain conversion. A schema change that breaks that alignment fails to compile.

## Tests

- Unit: `internal/...` and `pkg/...`, run by `make test` with `-race`. Use-case tests use the
  generated gomock mocks, are table-driven, and call `t.Parallel()` (`paralleltest` enforces it).
- Integration: `integration-test/` runs *inside* the docker network — it resolves the service as
  host `app` by container name, so it fails on the host machine.
  Always run it through `make compose-up-integration-test`.
