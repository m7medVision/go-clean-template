# go-clean-template

Go clean-architecture reference service. Three domains (`user`, `task`, `exchange`) exposed from
one shared use-case layer over each of these transports:

- REST — chi + Huma v2.
- gRPC. <!-- scaffold:grpc -->
- GraphQL via gqlgen at `/graphql` on the REST port, for tasks and the exchange history; GraphiQL <!-- scaffold:graphql -->
  at `/graphql/playground` when `DOCS_ENABLED`. <!-- scaffold:graphql -->

<!-- scaffold:queue:begin -->
A second binary, `cmd/worker`, consumes a minimal Postgres queue (the `queue_messages` table);
see **Queue and worker** below.
<!-- scaffold:queue:end -->
Module path: `github.com/m7medvision/go-clean-template`. The Go version is declared in `go.mod`.
A React client (TanStack Router + Query, built with Vite+) lives in `web/`; see **Frontend** below. <!-- scaffold:web -->

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
| Regenerate protobuf | `make proto-v1` | `protoc ...` | <!-- scaffold:grpc -->
| Regenerate SQL query code | `make sqlc` | `sqlc generate` — the target first renders the HCL schema to SQL with Atlas (needs Docker for `ATLAS_DEV_URL`), then runs the sqlc pinned in `go.mod` |
| Regenerate the REST OpenAPI document | `make openapi` | `go run ./cmd/openapi` — the target writes through a temp file so a failed run never truncates `docs/openapi/v1.json` |
| Install frontend dependencies | `make web-deps` | `bun install` / `vp install` — the target uses the frozen lockfile | <!-- scaffold:web -->
| Regenerate frontend REST types | `make web-api` | `npx openapi-typescript ...` — the target refreshes `docs/openapi/v1.json` first | <!-- scaffold:web -->
| Regenerate frontend GraphQL types | `make web-graphql` | `npx graphql-codegen` — reads the gqlgen schema and the operations in `web/src/` | <!-- scaffold:web -->
| Run the frontend dev server | `make web-dev` | `vp dev` — the target exports `HTTP_PORT`, which the Vite proxy targets | <!-- scaffold:web -->
| Format / lint / test the frontend | `make web-format`, `make web-check` | `vp check`, `vp test`, `tsc`, `eslint`, `prettier` | <!-- scaffold:web -->
| Build the frontend | `make web-build` | `vp build` | <!-- scaffold:web -->
| Regenerate GraphQL | `make graphql` | `go tool gqlgen ...` — `gqlgen.yml` paths resolve against the working directory, so the target runs gqlgen from the config's directory | <!-- scaffold:graphql -->
| Tidy / verify modules | `make deps` | `go mod tidy` |
| Vulnerability scan | `make deps-audit` | `govulncheck ./...` |
| Start dependencies (Postgres) | `make compose-up` | `docker compose up` |
| Start the whole stack including the app | `make compose-up-all` | `docker compose up` |
| Tear down | `make compose-down` | `docker compose down` |
| Run the app locally | `make run` | `go run ./cmd/app` — the target regenerates docs and builds with `-tags migrate` |
| Run the queue worker locally | `make run-worker` | `go run ./cmd/worker` | <!-- scaffold:queue -->
| Run everything in dev mode (tmux, hot reload) | `make dev` / `make dev-down` | `go tool air ...` in separate terminals — `scripts/dev.sh` passes the loaded env into every tmux pane and turns tracing off |
| Apply schema changes | `make schema-apply` | `atlas schema apply ...` |
| Full check before pushing | `make pre-commit` | running the steps by hand |
| Generate a project with only some features | `make scaffold` (form) or `make scaffold PRESET=api`; `scaffold-check`, `scaffold-dev`, `scaffold-matrix` | `go run ./cmd/create` — the targets pass `--force` for the `scaffold/out` scratch area | <!-- scaffold:self -->
| See what master would be released as | `make release-version`, `make release-notes` | tagging by hand — `release.yml` tags and releases on its own; see `docs/releasing.md` | <!-- scaffold:self -->

`make help` lists every target.

Traps in these targets:

- **`make integration-test` is not the one you want.** It runs `go test ./integration-test/...` on
  the host, where the suite cannot resolve the container hostnames it needs, so it always fails.
  `make compose-up-integration-test` is the real entry point.
<!-- scaffold:grpc:begin -->
- **`make run` and `make pre-commit` depend on `proto-v1`**, so they need `protoc` on `PATH`.
  Run `make bin-deps` first (`protoc` itself is not installed by it).
<!-- scaffold:grpc:end -->
<!-- scaffold:web:begin -->
- **`make pre-commit` also runs the frontend targets**, so it needs the `vp` CLI (Vite+) on `PATH`;
  `vp` provisions the Node version in `web/.node-version` and bun itself.
<!-- scaffold:web:end -->

Never claim a change is done without `make format`, `make linter-golangci` and `make test` passing.
For changes under `web/`, or to any Huma input/output struct, `make web-api` and `make web-check` too. <!-- scaffold:web -->
For changes to a GraphQL schema file or to an operation in `web/src/`, `make web-graphql` too. <!-- scaffold:web -->
For changes to markers or `scaffold/features.yaml`, `make scaffold-check WITHOUT=<feature>` too. <!-- scaffold:self -->
<!-- scaffold:self:begin -->
Commit subjects are Conventional Commits, and they name the release: a `feat`, `fix`, `perf` or
breaking (`!`) commit merged to master is released as the next version (`docs/releasing.md`);
`docs`, `ci`, `chore`, `test`, `build` and `refactor` release nothing. Never move or delete a `v*` tag.
<!-- scaffold:self:end -->

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
  (Postgres) and `internal/repo/webapi/` (outbound HTTP: the ExchangeRate-API rate client, plain `net/http`,
  unit-tested against the recorded response in `testdata/` that the integration stub also serves) implement them.
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
| a transport status or error code, a message envelope | `internal/controller/<transport>/v1/<domain>.go` | handler |
| a queue message (decode, call a use case, ack or retry) | `internal/controller/worker/v1/<name>.go` | `HandlerFunc`, registered in `v1/router.go` | <!-- scaffold:queue -->
<!-- scaffold:email:begin -->
| the data an email shows | `internal/entity/email_<name>.go` | struct with `Template()`/`Recipient()`, plus an entry in `EmailTypes()` (`email.go`) |
| an email's wording and markup | `internal/repo/mailtmpl/templates/<name>.{html,txt}.tmpl` | `content`, and `subject` + `text`; fixture and golden in `mailtmpl_test.go` |
<!-- scaffold:email:end -->
| a server, pool, client or middleware with no domain knowledge | `pkg/<name>/` | reusable package |

Layout rules:

- One file per domain per layer, named after the domain (`task.go`, `user.go`, `exchange.go`),
  plus a `tracing.go` in every `usecase/<domain>/` and `repo/persistent/<domain>/` package.
- Package name matches the directory name. A repository package shares its name with the use-case
  package of the same domain, so `internal/app/app.go` imports the repositories under aliases
  (`persistTaskRepo`, …).
- Constructors are always `New`, return the *interface* from `contracts.go`, and take their
  dependencies as arguments. No package-level state — `gochecknoglobals` and `gochecknoinits` are on.
- Unexported package constants are prefixed with an underscore (`_defaultEntityCap`,
  `_tracerName`). `mnd` rejects bare numeric literals, so add a constant rather than inlining one.

## The handler contract

Every controller method, on every transport, does the same six things in the same order. Copy the
shape from a neighbouring handler in the same transport.

1. **Get the caller.** REST: `middleware.UserID(ctx)` (the chi `middleware.Auth` middleware stores
   it in the request context).
   gRPC: `grpcmw.UserIDFromContext(ctx)`. <!-- scaffold:grpc -->
   GraphQL: `caller(ctx)`. <!-- scaffold:graphql -->
   A failure here is an auth error, not a 500.
2. **Decode into a transport-local DTO** from `<transport>/v1/request/`. On REST the DTO is the
   `Body` field of a small Huma input struct; never decode into an `entity` type, and never pass a
   `request.*` type into a use case.
3. **Validate the DTO structurally through Huma's built-in validation** on the REST transport:
   length, format, enum, and default constraints live on the input struct / request DTO tags, and
   the REST v1 controller takes no `*validator.Validate`.
   The gRPC controller validates its request DTOs with its own. <!-- scaffold:grpc -->
   Domain validation — anything that needs to know the rules, like a status transition —
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
   `response.Token`).
   gRPC always converts through `response.New*Response`. <!-- scaffold:grpc -->

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

**Money is `decimal.Decimal`, never `float64`.** `entity.Conversion` holds `shopspring/decimal`
values; sqlc maps every `numeric` column to it (`sqlc.yaml` override) and pgx reads and writes it
through `sql.Scanner`/`driver.Valuer`. Transports carry decimals as strings: Huma cannot build a
schema for `decimal.Decimal`, so REST returns `response.Conversion`, not the entity.

**Config is environment-only.** `config/config.go` parses env vars with `caarlos0/env`. A new
setting needs the struct field, an entry in `.env.example`, and — if the container needs it — the
`environment` block in `docker-compose.yml`. Fields tagged `required` make the app fail at
startup. `.env.example` is read both by `make` (`include`) and by the shell in CI
(`. ./.env.example`), so values there must not need quoting: no spaces.
`MAIL_FROM` is a bare address for that reason. <!-- scaffold:email -->

**The linter is strict and non-obvious.** `.golangci.yml` enables ~45 linters. The ones that
reject otherwise-fine code: `wsl_v5` and `nlreturn` (blank line required before `return` and
around blocks), `funlen` (65 lines / 40 statements), `gocyclo` (10), `gocognit` (15), `mnd`,
`gochecknoglobals`, `exhaustive` (switches over enum types need every case or a `default`), `dupl`
(threshold 100 — copy-pasting a handler across transports trips it).

## Generated files — do not hand-edit

- `docs/proto/v1/*.pb.go` ← `make proto-v1` from the `.proto` files beside them. <!-- scaffold:grpc -->
- `internal/usecase/mocks_*_test.go` ← `make mock`.
- `internal/repo/persistent/queries/schema/schema.sql` ← `make sqlc`, rendered by Atlas from
  `internal/app/schema/*.hcl`. It exists only because sqlc cannot read HCL; CI fails when it is stale.
- `internal/repo/persistent/queries/*.go` ← `make sqlc` from the `.sql` query files beside them,
  that rendered schema, and `sqlc.yaml`.
<!-- scaffold:graphql:begin -->
- `internal/controller/graphql/generated/` and `internal/controller/graphql/model/` ← `make graphql`
  from `internal/controller/graphql/schema/*.graphqls` and `gqlgen.yml`. The resolver bodies in
  `internal/controller/graphql/task.resolvers.go` are hand-written, but gqlgen rewrites that file in
  place on every run (signatures follow the schema, bodies are kept).
<!-- scaffold:graphql:end -->
<!-- scaffold:web:begin -->
- `web/src/api/schema.gen.ts` ← `make web-api` (openapi-typescript) from `docs/openapi/v1.json`.
- `web/src/graphql/` ← `make web-graphql` (GraphQL Code Generator, `web/codegen.ts`) from
  `internal/controller/graphql/schema/*.graphqls` and the `graphql(...)` operations in `web/src/`.
- `web/src/routeTree.gen.ts` ← the TanStack Router Vite plugin, on every `vp dev` / `vp build`.
<!-- scaffold:web:end -->
- `docs/openapi/v1.json` ← `make openapi`, exported by `cmd/openapi` from the same Huma config the
  server uses (`restapi.OpenAPI`). Huma still serves the live document at `/openapi.json` when
  `DOCS_ENABLED`; the checked-in copy exists so REST contract changes show up as a reviewable diff
  and so clients can generate types without a running server. CI fails when it is stale.

<!-- scaffold:queue:begin -->
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
- The worker is wired in `internal/app/worker.go` (`RunWorker`). Handlers that need use cases get
  them from `initUseCases`, like the servers. The Dockerfile builds `/app` and `/worker` into one
  image; the compose `worker` service runs `/worker`. Only the app applies the schema.
- A use case enqueues by taking `repo.QueueRepo` and calling `Send`.
<!-- scaffold:welcome:begin -->
- A use case can also enqueue through a port that hides the queue: the user use case calls
  `repo.UserNotifier.Welcome`, implemented by `internal/repo/notify`.
<!-- scaffold:welcome:end -->
<!-- scaffold:email:begin -->
- Email goes through the `mail` queue (`entity.EmailQueue`): the sender writes the body with
  `entity.EncodeEmail`, and `controller/worker/v1/mail.go` reads it with `entity.DecodeEmail`. That
  handler is the one exception to "decode into a transport-local DTO" — both ends are this codebase.
  The `mail` use case (`Deliver`) renders with `internal/repo/mailtmpl` and sends through `pkg/smtp`;
  only the worker calls it. `initUseCases` builds it for both binaries, and fails if a template is broken.
- Mailpit (compose service `mailpit`, UI and API on :8025) catches email in development and in the
  integration stack.
  `integration-test/mail_test.go` asserts on what reaches it. <!-- scaffold:welcome -->
<!-- scaffold:email:end -->
<!-- scaffold:queue:end -->

<!-- scaffold:web:begin -->
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

<!-- scaffold:web:end -->
<!-- scaffold:self:begin -->
## Optional features and scaffold markers

`make scaffold` (`cmd/create`, `internal/scaffold`, [docs/scaffold.md](docs/scaffold.md)) generates a
project with only some features, driven by `scaffold/features.yaml`. The template keeps every
feature; what it drops in a generated project is described by two things, which every change must
keep true:

- **A feature's own files are listed under its `paths`** in `scaffold/features.yaml`. A new file
  for an optional feature goes in a path it owns (its package, `config/<feature>.go`,
  `integration-test/<feature>_test.go`), or the manifest gets a new path.
- **Its lines in shared files sit between markers** in the file's comment syntax
  (`// scaffold:grpc:begin` … `// scaffold:grpc:end`, `# …` in Makefile/YAML/env, `<!-- … -->` in
  Markdown, or one trailing `// scaffold:grpc`). This includes docs prose: write lists, not counts
  ("every transport", not "three transports"). Mermaid and JSON cannot hold markers.
- Wire a feature through one call or one option (`restapi.WithPasskeys`, `workerctl.WithMail`,
  `internal/app/grpc.go`) so the shared files need few markers. A marked Go block between two
  top-level functions breaks the identity test after `gofumpt`; move that code to an owned file.

`make test` runs `internal/scaffold`'s repository tests:
- every file is claimed;
- markers are balanced and name real features;
- generating everything reproduces the template;
- a project without each feature contains none of its `leaks`;
- no project mentions the scaffolder.

`make scaffold-check WITHOUT=<id>` proves a project without a feature passes `make pre-commit`; CI
runs `scaffold-check` for every preset.

<!-- scaffold:self:end -->
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
| Queue or worker behaviour | `docs/worker.md` | <!-- scaffold:queue -->
| Email behaviour | the `Email` section of `docs/worker.md` | <!-- scaffold:email -->
| Frontend stack or conventions | `docs/web.md` | <!-- scaffold:web -->
| Scaffold manifest, markers or `make scaffold*` targets | `docs/scaffold.md` | <!-- scaffold:self -->
| Release rules, `release.yml` or `cmd/release` | `docs/releasing.md` | <!-- scaffold:self -->
| The exchange `Convert` path (handler, use case, entity, webapi, repo) | `docs/request-flow.md`, which quotes that code |
| A layering rule (what may import what, where code goes) | `Rules` in `docs/architecture.md`, and the `Clean Architecture` section of `README.md` |

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
     (op registered in the same domain file), then `make openapi`, and commit the generated diff.
     Then `make web-api`, commit its diff too, and fix any `make web-check` errors in `web/`. <!-- scaffold:web -->
   <!-- scaffold:graphql:begin -->
   - GraphQL (task and exchange today): schema change in `internal/controller/graphql/schema/` →
     `make graphql` → fill the generated resolver stub in `<domain>.resolvers.go`.
     Then `make web-graphql` and fix any `make web-check` errors in `web/`. <!-- scaffold:web -->
   <!-- scaffold:graphql:end -->
   <!-- scaffold:grpc:begin -->
   - gRPC: `.proto` change → `make proto-v1` → controller method in
     `internal/controller/grpc/v1/`.
   <!-- scaffold:grpc:end -->

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
   added to the `V1` / controller struct in `<transport>/v1/controller.go`.
   gRPC also needs `docs/proto/v1/<domain>.proto` and `make proto-v1`. <!-- scaffold:grpc -->
10. **Docs** — `make openapi` to refresh `docs/openapi/v1.json`, then the docs updates listed
    above.
11. **Integration test** — `integration-test/<domain>_test.go` exercising the domain over the
    transports you exposed.

## API versioning

Routes are grouped under a version package (`<transport>/v1/`). A v2 means a new sibling package
with the same layout, registered alongside v1 in the transport's top-level `router.go` — REST adds
operations under the new `/v2/...` paths from a `v2.NewRoutes(api, ...)` in `restapi/router.go`.
gRPC registers a second set of `NewXRoutes`. <!-- scaffold:grpc -->
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
`internal/repo/persistent/queries/schema/schema.sql`; edit the HCL, never that file. The repositories call the generated
`*queries.Queries` and convert its rows to `entity` types; there is no ORM and no query building
at runtime. Generated row structs share the entities' field order, so `entity.Task(row)` is a
plain conversion. A schema change that breaks that alignment fails to compile.

## Tests

- Unit: `internal/...` and `pkg/...`, run by `make test` with `-race`. Use-case tests use the
  generated gomock mocks, are table-driven, and call `t.Parallel()` (`paralleltest` enforces it).
- Integration: `integration-test/` runs *inside* the docker network — it resolves the service as
  host `app` by its compose service name, so it fails on the host machine.
  Always run it through `make compose-up-integration-test`.
<!-- scaffold:email:begin -->
- Mail templates: `internal/repo/mailtmpl/testdata/*.golden.*` are compared by the render test;
  after an intended template change, `UPDATE_GOLDEN=1 go test ./internal/repo/mailtmpl/` and review the diff.
<!-- scaffold:email:end -->
