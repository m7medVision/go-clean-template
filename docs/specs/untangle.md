# Untangling the Go service — decision log

Scope: the Go code only (`cmd/`, `config/`, `internal/`, `pkg/`, `integration-test/`, `tools/gen`).
`web/` is touched only where a generated file follows a Go change (`make web-api`, `make web-graphql`).
Extras chosen: **Comments** and **Tests**. Approved 2026-11-07; done: merged 2026-11-07 (#51), released in v0.3.1. Plan: [untangle-plan.md](untangle-plan.md) ·
Tasks: [untangle-todo.md](untangle-todo.md).

Method: an import graph, a skeleton of all 7.7k non-test declaration lines, git co-change history,
six read-only reviews (entity+usecase, repo, transports, app+pkg, generator, tests), and spot checks
of the claims the big fixes rest on (each marked ✔ below where I verified it myself).

The question asked of every piece: **should these pieces change together?**

---

## The fixes, each above the problems it solves

### A. One error table: each sentinel carries its kind; each transport maps kinds once

`entity.Err*` become typed (`entity.Error{Kind, Msg}`; kinds Invalid, Unauthenticated, Forbidden,
NotFound, Conflict, Unavailable, NotImplemented; anything untyped is Internal). REST, gRPC and GraphQL
each get one `kind → status` function that also logs Internal once. Domain folders stop mapping errors.

- [HIDDEN] The same sentinel→status mapping is written 8 times, with the same messages: `restapi/v1/task/controller.go:54`, `restapi/v1/user/controller.go:33`, `restapi/v1/exchange/controller.go:32`, `restapi/v1/oidc/oidc.go:99`, `grpc/v1/task/controller.go:46`, `grpc/v1/user/controller.go:34`, `grpc/v1/exchange/controller.go:32`, `graphql/gqlerr/gqlerr.go:46`. ✔ A new sentinel that one of them misses becomes a 500.
- [HIDDEN] `restapi/v1/passkey/passkey.go` maps errors inline in 7 blocks (66, 107, 146, 190, 227, 274, 301).
- [HIDDEN] The three "any command" sentinels and their messages are copied into `apierr.go:17`, `rpcerr.go:17` and `gqlerr.go:52-55`. The order differs (GraphQL checks domain errors first), and so does logging (REST/gRPC Warn, GraphQL/passkey silent).
- [HIDDEN] `gqlerr` is a shared package holding the task domain's sentinels (`gqlerr.go:48-51`) ✔. Exchange's are missing, so an exchange mutation would answer INTERNAL.
- [SPECIAL] Passkey logs expected 4xx at Error (`passkey.go:67,112,195,232,306`) and skips `apierr.Shared` in 3 ops (66, 146, 274).
- [SURPLUS] `ErrRoleRequired` (`entity/errors.go:14`) is unreachable, `Authorize`'s `if RequiredRole()==RoleAdmin` (`behavior.go:128`) exists only to choose between it and `ErrAdminRequired`, and no transport maps it.
- [HIDDEN] OpenAPI `Errors` lists are hand-kept and wrong: no operation lists 503/409/501, which every command can return (`"503"` occurs 0 times in `docs/openapi/v1.json`). Derived from the table, they stay right.
- [HIDDEN] The generator relies on per-domain `problem`/`fail` methods (`tools/gen/usecase_rest.go:24-26`, `usecase_grpc.go:34-36`) and inserts into `gqlerr.Map` by anchoring on `entity.ErrReadModelBehind` (`tools/gen/layer_graphql.go:51`).

### B. Input rules live in the domain; read models stop re-checking them

Commands and queries implement `Validate()` (the validate step already runs it) for title/description
lengths, sign-up rules, page bounds and the amount format; aggregates normalize their own values
(`PasskeyName`). One `entity.Page` (default 10, max 100) for every list. Read-model columns stop
enforcing what the domain enforces (`varchar(255)` → `text`, the unique indexes on `users` → plain
indexes). Transports keep their schema hints (Huma tags, GraphQL defaults) for docs, and one test pins
them to the entity constants, so the link is visible.

- [HIDDEN, bug] gRPC `Register`, `CreateTask` and `UpdateTask` send input unchecked (`grpc/v1/user/user.go:18`, `grpc/v1/task/task.go:22,104`) ✔, and `RegisterUser` has no `Validate()` ✔. Over gRPC you can sign up with an empty email and password.
- [HIDDEN] The task title (1–255) and description (≤1000) rules exist only in `restapi/v1/task/request.go:9-18` and `graphql/task/input.go:13-17`. Sign-up rules exist only as Huma tags (`restapi/v1/user/request.go:7-9`).
- [HIDDEN, bug] A 256-character title over gRPC, or a long OIDC `preferred_username` (`usecase/oidc/sso/sso.go:140`), commits as an event. The projector insert then fails on `varchar(255)` (`schema/tasks.hcl:15`, `users.hcl:9`), and the runner retries it forever (`projection/runner.go:201-224`). Because of D below, every write in every domain then answers `ErrReadModelBehind`.
- [HIDDEN] `users.hcl:32-39` keeps UNIQUE indexes on username and email ✔, while uniqueness is a claim (CLAUDE.md says "not a unique index on a read model"). If the two ever disagree, the result is a stuck projector, not a rejected write.
- [SURPLUS] The `Validate` step and `Validator` interface (`behavior.go:29-33,136-147`) have no implementers ✔.
- [SURPLUS] The gRPC task and user controllers build a `*validator.Validate` that nothing uses (`grpc/v1/task/controller.go:26,31`, `grpc/v1/user/controller.go:24,29`), and the template copies it.
- [HIDDEN] Paging is written in 7 places that disagree:
  - REST task has no max (`restapi/v1/task/task.go:26`); REST exchange has max 100 (`restapi/v1/exchange/exchange.go:26`).
  - GraphQL has `_maxLimit` (`graphql/exchange/query.go:16`), plus `task.graphqls:20,22` and `exchange.graphqls:18`.
  - gRPC has none (`grpc/v1/task/task.go:66`, `grpc/v1/exchange/exchange.go:55`).
  - The normalization is copied in `list_tasks.go:48-54` and `get_exchange_history.go:34-39`.
  - The page type is `query.TaskPage` for tasks but `entity` for exchange; the contracts take `TaskFilter` in one and `limit, offset` in the other.
- [HIDDEN] The amount rule differs: REST checks a regex (`restapi/v1/exchange/request.go:10`), while gRPC uses `decimal.NewFromString` (`grpc/v1/exchange/exchange.go:24`). REST answers 422 but its op lists 400 (`exchange.go:43,52`).
- [HIDDEN] `PasskeyName` is normalized by handlers (`rename_passkey.go:43`, `finish_passkey_registration.go:55`), not by the aggregate, so a new caller would record blank names.
- [HIDDEN] The task status values are written 7 times: `entity/task.go:11-13,31,40-48`, the enum tags in `restapi/v1/task/task.go:25` and `request.go:25`, `task.graphqls:1` and `gqlgen.yml:24-32`. Entity owns the list; the copies get pinned by the test.
- [HIDDEN] `graphql/task/input.go:5` points at `restapi/v1/request/task.go`, which does not exist.

### C. The write model owns its state; the read shape is only the read shape

Each aggregate keeps a private state struct with its own snapshot encoding. `entity.Task`, `User` and
`Passkey` stay the read shape: API, gqlgen model and sqlc row, which do change together. Aggregates that
are never loaded drop the state and snapshots nobody reads.

- [HIDDEN] `entity.Task`, `User` and `Passkey` have five jobs: Huma schema, gqlgen model, sqlc row, aggregate state and snapshot encoding (`task_aggregate.go:131-134`). Renaming an API JSON field changes stored snapshots with no version bump. `json:"-"` forces side fields into `userSnapshot` (`user_aggregate.go:121-135`) and `passkeySnapshot` (`passkey_aggregate.go:142-173`).
- [SURPLUS] `UserAggregate` and `ConversionAggregate` are never loaded ✔ (no `aggregate.Change` on them). Their snapshots, the `identities` state (`user_aggregate.go:35,111,117`) and `State()` are used only by tests. `exchange_aggregate.go:54-63` repeats the same `Apply` case twice.
- [HIDDEN] The passkey ownership rule is copied into `finish_passkey_login.go:66`, because `ownedBy` is unexported. `Use` (`passkey_aggregate.go:95`) checks only `Live`.
- [HIDDEN] The OIDC identity claim is built twice: `user_aggregate.go:87-88`, and by hand at `finish_oidc_login.go:73-75,91`. The race handling at `:89` depends on that claim answering `ErrUserAlreadyExists` while the name claims answer `ErrAccountConflict` (`:60`).
- [SURPLUS] `aggregate.Change` takes 8 parameters. Five callers pass `id`, `XStreamID(id)`, `&XAggregate{}` and a not-found error, all of which follow from the aggregate type. With generics it becomes `aggregate.Change[*entity.TaskAggregate](ctx, store, id, fn)`. `aggregate.Create`'s `id` repeats what the aggregate already knows.
- [HIDDEN] Event schema versions are one constant per domain (`task_events.go:7`, `user_events.go:6`, `exchange_events.go:11`, `passkey_events.go:10`), but upcasts are per event. Changing one task event re-versions all five. Each event should carry its own version, starting at today's value, so nothing stored changes.

### D. Messages declare their result; the bus derives their names; `Written` leaves `pkg/cqrs`

A message embeds `cqrs.Command[R]` or `cqrs.Query[R]`, and `cqrs.Send(ctx, bus, msg)` infers `R`. I
verified in a scratch module that Go infers it. The name used in logs and metrics comes from the Go type.
`Written` (aggregate ID, position, event types written) moves out of the generic bus into the use-case
layer, and the wait step reads it through a named interface. Commands that write no events, like
`DeliverEmail`, return `struct{}`, so there is no "position 0 means don't wait" convention.

- [HIDDEN] All 47 `cqrs.Send*[R]` calls restate the handler's result type. A mismatch is `ErrResultType`, a 500 at runtime (`pkg/cqrs/bus.go:107`).
- [HIDDEN] 25 `CommandName`/`QueryName` methods copy the type name as a string ✔ (26 checked, all equal), and renaming the type leaves the string stale.
- [NEEDLESS] `Written{ID, Position}` puts the event store's position inside the "generic" bus (`pkg/cqrs/cqrs.go:46-55`).
- [HIDDEN] The wait step finds the position by duck typing on an inline `interface{ WrittenAt() int64 }` (`behavior.go:61`). Renaming the method silently turns waiting off. "Position 0 = don't wait" is used at `finish_oidc_login.go:77,96` and `deliver_email.go:41`.
- [SPECIAL] `DeliverEmail` returns an empty `Written` only to fit (`deliver_email.go:41`), and `mail.Register` has its own signature (`mail/register.go:17`).
- [SURPLUS] `pkg/cqrs/bus.go:73-76` is unreachable, and the metrics `AlreadyRegistered` path (`metrics.go:51-63`) is never hit.

### E. Projections declare what they handle; one codec, shared by both stores

A projector is a value (`projection.Projector{Name, Tables, Handles, Project}`), not a duck type:

- The runner skips the event types a projector doesn't handle, without a transaction per event.
- Startup fails if an event type in `EventTypes()` has no projector and isn't declared as ignored.
- The wait step waits only on the projectors that handle the event types the command wrote, so a stuck passkey projector no longer blocks task writes.

The codec, the upcast registry, the claim rules and `SnapshotDue` move into a pgx-free package that the
Postgres store and `memstore` both use. Upcasts sit next to the events. `ErrClaimTaken` moves into
`entity`. The app builds one codec, once.

- [HIDDEN] Projectors are told apart from event handlers by whether they have a `Tables()` method (`projection/rebuild.go:25-28,43`, `app/buses.go:135`). A projector that forgets `Tables()` silently stops being waited on and can't be rebuilt.
- [HIDDEN] A projector's type switch has no default and no list of the types it handles (`persistent/task/projector.go:45`, and the same in the other three). A new event type with no projector case compiles and is silently dropped.
- [NEEDLESS] Every event is delivered to every subscriber, each in its own transaction with a try-lock and a checkpoint UPDATE (`runner.go:212-237`), even for events it ignores.
- [NEEDLESS] Every command waits on every projector (`buses.go:65`) ✔, so `RegisterUser` waits on the task projector.
- [HIDDEN] The app builds the notifier and queue only to read the subscribers' names (`app.go:36`, `buses.go:114-122`). The worker builds the same list on its own (`worker.go:95`). If the two builds differ, writes wait on a projector that never runs.
- [HIDDEN] A projector's `Name()` is a stored key (checkpoint row, rebuild argument, wait list). The same literal is repeated as the importer's `Domain()` (`task|user|passkey/importer.go:29-30`, `exchange/importer.go:40`).
- [SURPLUS] Every projector repeats the same struct, `NewProjector`, `Name`, `Tables` and `Handle` (about 35 lines × 4). `Project` is exported only for its tests.
- [SURPLUS] `projection/postgres.go:44-51` is a pass-through of `ReadAll`/`InTx`. `projection.Store` and `projection.Reader` each have one implementation. `Runner.Subscribers()` (`runner.go:89`) has no caller.
- [NEEDLESS] `memstore` imports `persistent/eventstore` for its codec, which pulls pgx, sqlc and OTel into every command-handler test (`memstore.go:15`).
- [HIDDEN] `memstore` has drifted from Postgres:
  - claim ownership is coded twice (`memstore.go:126-154` vs `eventstore.sql:59-64`);
  - it checks the version even when nothing is pending (`:82` vs `eventstore.go:42`);
  - it has no trace-correlation fallback (`:108` vs `eventstore.go:164`) and no upcasts (`:61`);
  - it swallows decode errors (`:232`).
- [HIDDEN] Upcasts live in the Postgres adapter (`eventstore/upcasts.go:6`) while `SchemaVersion` lives in `entity`. `EventTypes()` and `Upcasts()` are wired separately in `buses.go:75` and `worker.go:100`, which build two codecs.
- [SPECIAL, kept] A failing event is retried forever (`runner.go:145-151`). That is right for a projector bug. B removes the known causes of a stuck projector, and the scoped wait limits the damage. Lag is already exported as a metric.

### F. Protocol adapters and session issuing sit behind ports, in the outer layer

`usecase/oidc/sso` and `usecase/passkey/ceremony` are adapters for go-oidc and go-webauthn. They move
under `internal/repo/`, behind ports in the contracts. Token issuing gets one port over `pkg/jwt`, and
every sign-in returns one `entity.Session`. Password hashing and checking live on `entity.User`. Scope
labels, lifetimes and clocks move from `internal/app` into the adapters that use them.

- [NEEDLESS] Use cases import protocol libraries through concrete `sso` and `ceremony` types (`oidc/register.go:16`, `passkey/register.go:14`). Faking them for handler tests takes 279 lines of non-test packages (`oidctest`, `passkeytest`).
- [NEEDLESS] `ceremony.Started` leaks into `restapi/v1/passkey/controller.go:10`, while OIDC uses `query.Started` for the same idea.
- [NEEDLESS] Three domains depend on the concrete `*jwt.Manager` (`user/register.go:15`, `oidc/register.go:18`, `passkey/register.go:20`), converting roles through `RoleNames`.
- [HIDDEN] Token issuing is copied 3 times (`log_in.go:44`, `finish_oidc_login.go:63`, `finish_passkey_login.go:90`), with three result shapes: a bare string and two identical `SignedIn` structs.
- [HIDDEN] The bcrypt scheme is split between `register_user.go:36` (hash) and `log_in.go:40` (compare), in different packages.
- [HIDDEN] The scope labels and lifetimes live in `internal/app`:
  - `oidc.go:13,17` and `passkey.go:12,17,26`;
  - `CeremonyTimeout` is passed to two constructors that must agree, and OIDC's equivalent is a hard-coded constant;
  - the tests copy the scope strings (`passkeytest.go:21`, `finish_oidc_login_test.go:38`).
- [HIDDEN] Two clocks, neither set in production: `aggregate.Now` and `ceremony.go:88-91`. `aggregate.Now` builds Postgres timestamp precision into the use cases (`aggregate.go:52-58`).
- [HIDDEN] `FinishPasskeyLogin` passes state through callback side effects:
  - `lookup` sets `used`, and `record` reads it with a nil-check special case (`finish_passkey_login.go:57-78,102`);
  - `ceremony.validateLogin` does the same with `lookupErr` (`ceremony.go:228-245`);
  - it calls `aggregate.Create` on an aggregate it loaded (`:123`).
- [SPECIAL] Passkey breaks the patterns the other domains follow: load-or-new by hand (`finish_passkey_registration.go:50-53`), registration by pointer, and `Register(*Deps)` (`passkey/register.go:24,28`).
- [SPECIAL, kept] `FinishPasskeyLogin` opens its own transaction (`:108`). Burning the challenge and recording the use must commit together, so this is a real requirement. It keeps its port.

### G. One place per domain at the composition root; one bootstrap; one lifecycle; config by section

`internal/app` gets one file per domain that builds everything for it: read model, handlers,
projectors, importers, and its REST, gRPC and GraphQL registration. Both binaries iterate one `modules`
list. Each binary installs only what it uses: the worker installs mail, the app the rest.

A shared `bootstrap` (logger, tracing, pg, store, buses) returns errors instead of calling Fatal. Servers
are plain `http.Server` and `grpc.Server`, and they run, together with the worker and the runner, as
`Run(ctx) error` under one errgroup with `signal.NotifyContext`.

Config section types are exported and passed by section. Defaults live only in config. Migrate takes
`cfg.PG.URL`, and `cmd/openapi` needs no env. The app builds one chi mux that mounts REST and GraphQL
explicitly.

Domain lists, today ✔:

| Where | Lists the domains |
| --- | --- |
| `app/buses.go` | `registerHandlers` |
| `app/buses.go` | `subscribers` |
| `app/backfill.go` | the importers |
| `restapi/v1/router.go` | the REST routes |
| `restapi/openapi.go` | the REST routes again |
| `grpc/v1/router.go` | the gRPC services |
| `graphql/resolver.go` | the GraphQL resolvers |

After G, only `modules` and `graphql/resolver.go` remain. gqlgen needs one root type that embeds every
domain's resolver, so `resolver.go` stays.

- [HIDDEN] The domain is listed in the 7 places in the table above.
- [HIDDEN] `restapi/openapi.go:17-24` repeats `app.go:45-49`, and `router_test.go:173-178` hard-codes the paths a third time.
- [SURPLUS] The `restapi.WithPasskeys/WithOIDC` and `WithMail` options are leftovers from the removed scaffolder. Production always passes them (`app.go:47-48`, `app/worker.go:70`).
- [SURPLUS] Routers sit at two levels (`grpc/router.go` + `grpc/v1/router.go`, `worker/router.go` + `worker/v1/router.go`).
- [SURPLUS] 11 controller structs hold only the buses and a logger.
- [NEEDLESS] Both binaries build every dependency:
  - the worker builds the passkey ceremony, SSO and a JWT manager it never uses, so bad `WEBAUTHN_*` settings stop the worker;
  - the app builds SMTP and the templates (`mail.go:18-31`) even though only the worker sends.
- [SURPLUS] The `Run` and `RunWorker` preambles are near-identical (`app.go:81-104`, `worker.go:24-45`). Signal handling is written twice. The user read model is built twice (`buses.go:99,104`), the event store twice (`app.go:37,107`), and `subscribers` twice in the worker.
- [SPECIAL] Nine `l.Fatal` calls (`app.go:89,102,108,116,122`, `worker.go:32,43,50,56`) skip the deferred `pg.Close` and the trace flush.
- [SURPLUS] `pkg/httpserver` and `pkg/grpcserver` wrap the standard servers. They carry three options with zero callers (`options.go:27-46`), `Port`/`Handler` passed as options although they are required, an unused `:80` default, and an errgroup with `SetLimit(1)` guarding one goroutine.
- [SPECIAL] The wrappers also have lifecycle bugs:
  - a listen failure is logged 3 times;
  - Shutdown takes its context from an already-cancelled errgroup (`server.go:95`);
  - `Notify` goroutines leak on a clean exit (`servers.go:38-42`);
  - `servers.go:40,47,54` hard-codes the "app - Run" log prefix.
- [SURPLUS] `pkg/worker` `Shutdown` always returns nil, so `app/worker.go:82-84` is dead. `Worker.Queues()` is used only by tests.
- [HIDDEN] Call-order rules:
  - the store must close after the runner, which is why `newRunner` returns the runner's internal store (`worker.go:73-87`);
  - `Handle` must run before `Start`, with no lock (`pkg/worker/worker.go:96`).
- [NEEDLESS] `restapi.NewRouter` takes the whole `*config.Config` for 4 fields (`router.go:13,27`). So:
  - a controller package imports config;
  - `resttest.Main` must set `PG_URL`/`JWT_*` (`resttest.go:26-36`);
  - 5 `main_test.go` files exist only to call it;
  - adding a `required` setting breaks every REST test package.
- [NEEDLESS] The config section types are unexported, so every helper in `internal/app` takes the whole cfg, and three mirror structs copy sections field by field (`app.go:64-78`, `passkey.go:22-28`, `oidc.go:30-37`).
- [HIDDEN] `app/migrate.go:28-33` reads `PG_URL` from the env directly. It appends `?sslmode=disable` (which breaks a URL that already has a query), and it has its own wait loop, duplicating `pkg/postgres` (`postgres.go:51-62`).
- [HIDDEN] `cmd/openapi` needs `PG_URL` and `JWT_SECRET` just to read the version.
- [HIDDEN] Defaults are written in up to four places:
  - worker settings in `pkg/worker/worker.go:21-24`, `config/worker.go:8-11`, `.env.example` and `docker-compose.yml:37-40`;
  - projection settings in `runner.go:20-21` and `config/cqrs.go`;
  - two have already drifted from `.env.example`: the `WEBAUTHN_RP_NAME` default, and `MAIL_FROM`.
- [HIDDEN] `TRACING_ENABLED` is read in two places with different meanings (`app.go:66`, `restapi/router.go:46`). `METRICS_ENABLED` gates `/metrics` but not `cqrs.Metrics`. Both are fixed by reading each once and passing a provider or registry that is a no-op when off.
- [HIDDEN] `BaseURL` normalization is written twice (`buses.go:126`, `oidc.go:34`).
- [HIDDEN] `httpserver`'s fixed 5s WriteTimeout (`server.go:17`) must exceed `CQRS_CONSISTENCY_TIMEOUT`; raising the env var past it silently truncates responses. Derive it.
- [NEEDLESS] GraphQL is mounted onto the REST router (`app/graphql.go:13`), so it inherits REST's middleware by accident. It escapes REST auth only because `/graphql` is outside `/v1/`. `DOCS_ENABLED` is checked twice.
- [NEEDLESS] gRPC auth is wired in `internal/app` (`grpc.go:20`), REST auth in the controller, and GraphQL auth in `NewHandler`.
- [SURPLUS] The `useCases` struct wraps one field (`app.go:22-28`).

### H. One token reader for every transport; "public" declared once per operation

One function turns a token into a `Principal`, with `issuedAt`, under one context key, for REST, gRPC
and GraphQL; each transport only reads its own header. REST enforces auth from each operation's Huma
`Security`, so the docs and the enforcement can't drift. gRPC lists its public methods by their
generated `FullMethodName` constants. GraphQL resolvers keep asking for the caller. gRPC gets the
panic recovery the other two transports already have.

- [HIDDEN] The JWT is parsed into a Principal 3 times (`restapi/middleware/auth.go:94`, `grpc/middleware/auth.go:59`, `graphql/middleware/auth.go:41`), with 3 context keys and 7 accessors. Only REST keeps `issuedAt`, so passkey re-auth can't work on the other transports.
- [HIDDEN] Which operations are public is stated 4 ways:
  - REST path prefixes (`auth.go:16-17,66`);
  - Huma `Security` on each operation;
  - a gRPC string allow-list that hard-codes `"/grpc.v1.AuthService/Register"` (`grpc/middleware/auth.go:34-37`);
  - a `Caller()` check in each GraphQL resolver.
- [HIDDEN] `_apiPrefix = "/v1/"` also gates tracing (`restapi/middleware/tracing.go:22`). A `v2/` (as `docs/api.md` describes) would be served without auth or traces, and `/graphql` is untraced today.
- [SURPLUS] 25 handler-level caller checks can never fail (15 REST `Error401Unauthorized`, 10 gRPC `!ok`), because the middleware already rejected the request.
- [SPECIAL, bug] gRPC has no panic recovery, so a panic kills the process. It also has no request log or metrics (`app/grpc.go:20`).

### I. Drop the hand-written tracing decorators

`cqrs.Trace` already makes a span per message, otelpgx one per query (`pkg/postgres/postgres.go:49`)
and otelhttp one per outbound call. The 9 `tracing.go` decorators add a fourth layer by hand.

- [SURPLUS] 9 decorators, 553 lines, 21 wrapped methods. `startSpan`/`endSpan` is copied 7 times. The generator keeps one more copy as a template plus a golden. CLAUDE.md's "a contract method lives in three places" rule exists only because of them.
- [HIDDEN] The span names have drifted from the methods: `task/tracing.go:41` names `Get` "GetByID", `exchange/tracing.go:43` names `History` "GetHistory", and passkey mixes "PasskeyRepo.*" with "PasskeyReadModel.Get".
- [NEEDLESS, bug] `user/tracing.go:48` writes the user's email into a span attribute ✔ (PII in traces).
- [SURPLUS] `passkey/tracing.go:47-54` wraps `GetByID`, which is in no contract and can't be reached.
- [SURPLUS] `webapi/tracing.go:30` opens a client span directly around otelhttp's own client span.
- [SPECIAL] The decorators wrap different things: `projection/checkpoints.go:20` returns an untraced concrete type, and `exchange` and `passkey` wrap `*Repo` where the others wrap the interface.

### J. Contracts per domain and per consumer; one way to join a transaction

`internal/repo/contracts.go` splits into one file per domain, with mockgen per file. Ports no use case
consumes leave the use-case contracts: `QueueRepo` becomes `worker.Queue` plus `notify`'s own `Send`, and
`Importer` moves to `eventstore`. Mailer and Renderer stop speaking `smtp.Message`. Sentinels move into
each domain's entity file. Every read model takes `Executor(ctx)`, and `pg.Pool` stops being exported.
The queue maps its row explicitly instead of converting it to `worker.Message`.

- [HIDDEN] `contracts.go` holds 15 ports for every domain and for the infrastructure, and `entity/errors.go` holds every domain's sentinels. The generator edits both for every domain.
- [SURPLUS] 7 of the 15 generated mocks are never constructed: QueueRepo, EventStore, Subscriber, Importer, ProjectionCheckpoints, ChallengeGuard and Transactor.
- [NEEDLESS] `contracts.go` imports `pkg/worker` only so `QueueRepo` matches `worker.Queue` (`:111-115`), and `pkg/smtp` for `MailRenderer`/`Mailer` (`:125-133`).
- [HIDDEN] Whether a repo joins the transaction in `ctx` depends on which constructor it used:
  - task, user and exchange use `pg.Pool` (`task.go:25`, `user.go:25`, `exchange.go:25`);
  - passkey uses `Executor(ctx)`;
  - `queue` mixes both (`queue.go:29` vs `:35`);
  - `pg.Pool` is reached into from `projection/postgres.go:116`.
- [HIDDEN] `worker.Message(row)` (`queue.go:56`) shapes a `pkg/` type by an internal table's column order.

### K. Replace `pkg/logger` with `log/slog` (decided: yes — the first, already decided step of the logging plan; nothing else from that plan)

- [SPECIAL, bug] `logger.go:95` uses `err.Error()` as a printf format, at 21 `l.Error(err, …)` call sites. A `%` in an error garbles the line.
- [SPECIAL, bug] `skipFrameCount=3` (`logger.go:29`) is right only for Debug/Error/Fatal, so Info and Warn report the wrong caller. `:99` prints the value where it means to print the type.
- [SURPLUS] The thin printf facade forces 5 hand-written fake loggers (`cqrs/steps_test.go:126`, `restapi/middleware/auth_test.go:25`, `resttest.go:108`, `grpc/v1/task/task_test.go:35`, `graphqltest.go:87`). slog has a discard handler.
- [HIDDEN] `pkg/postgres/postgres.go:57`, `tx.go:62` and `app/migrate.go` log through std `log`, bypassing the logger.
- [NEEDLESS] `pkg/cqrs` depends on `pkg/logger`.
- Tests: `logger_test.go:29,56,135` use private fields; `:103` asserts the opposite of its name; `:167` has an inverted `!ok`, so the exit-code assertion never runs. They go with the package.

### L. The backfill from v0.2.0 (decided: keep it, isolated — upgrading from v0.2.0 keeps working)

Either way, the `*Imported` events, their `Apply` cases and their projector cases stay, because events are forever.

**Keep:**

- markers move out of `projection_checkpoints`;
- one generic import loop in `eventstore.Backfill`, which also handles skip-on-conflict once;
- importers stop exporting functions for tests;
- `Rebuild` refuses while an import is pending.

**Retire** (v0.4.0; anyone on v0.2.0 upgrades through v0.3.x first): delete the 4 importers, their
tests, `Backfill` and the markers. With them go `user_identities`, a table only the importer reads,
and `include_scratch` if the eventstore suite can run in a rolled-back transaction.

The problems either way:

- [SPECIAL] A one-time migration runs at every start (`app.go:107`) and takes the global append lock 4 times. It reaches about 30 files.
- [SPECIAL] All 4 importers swallow `ErrConcurrencyConflict` (`task/importer.go:53`, `user:69`, `exchange:82`, `passkey:53`). That is safe only because `checkVersion` runs first: a nested `InTx` joins without a savepoint (`pkg/postgres/tx.go:50-52`).
- [HIDDEN] Importers read the read-model tables as their source, and `exchange/importer.go:59` writes one (`SetConversionID`). `Rebuild` truncates those tables (`rebuild.go:65`) without checking whether an import is pending.
- [HIDDEN] The underscore convention is spread over 5 places: `eventstore.sql:30,37,106,109`, `backfill.go:12`, `buses.go:150`, `eventstoretest`, and `projection_checkpoints.hcl:1-3`. Because markers share the checkpoints table, every checkpoint query filters them.
- [SURPLUS] `user_identities` is written by the projector (`user/projector.go:64-96`), but its only reader is the importer.
- [SPECIAL] `include_scratch` (`eventstore.sql:30`, `reader.go:17,28`) is a production SQL parameter that exists only for `integration-test/projection_test.go:97`.

### M. Worker and queue: one handler list, no demo queue in production, bounded handlers

- [SURPLUS] The `example` queue and its one-struct `worker/v1/request` package poll Postgres every second in production. Only `integration-test/queue_test.go:18` sends to it. The queue tests can go through `mail` instead.
- [SURPLUS] `worker/v1/controller.go` is 10 lines; there are two router levels and a `WithMail` option.
- [SURPLUS] `pkg/worker` and the projection runner each write the same poll/wake/shutdown loop and the same `PollInterval`/`BatchSize` options (`worker.go:140-151`, `runner.go:142-158`). Both get one `Run(ctx) error` shape (G).
- [SPECIAL, bug] A handler's context isn't bounded by the visibility timeout, so a slow handler is redelivered while it is still running.
- [SURPLUS] `queue.Delete` ignores its queue argument (`queue.go:64`).
- [SPECIAL] `smtp` parses `MAIL_FROM` on every send (`smtp.go:60`). A bad value shows up only at delivery and redelivers forever; it should fail at startup.

### N. Small fixes found along the way

- [SPECIAL, bug] `pkg/postgres/postgres.go:98-101` `safeIntToInt32` masks bits instead of clamping ✔ (2³¹ becomes 0).
- [HIDDEN] gRPC int32 clamps are written twice, differently (`grpc/v1/task/response.go:30` vs `exchange/response.go:37`).
- [HIDDEN] gRPC formats timestamps with a literal `"…Z"` layout 6 times, without `.UTC()` (`grpc/v1/task/response.go:18-19`, `user/response.go:14-15,25-26`).
- [HIDDEN] The conversion's 2 decimal places are copied 3 times (`restapi|grpc|graphql/.../exchange/response.go`), mirroring the unexported `entity/exchange.go:11 _resultScale`. Entity should export the formatting.
- [SURPLUS] Identical output types: `createTaskOutput`, `updateTaskOutput` and `transitionTaskOutput`, and `getTask` even returns `updateTaskOutput` (`restapi/v1/task/task.go:20,54,63,158`). In gRPC, `NewRegisterResponse` equals `NewGetProfileResponse` (`user/response.go:9,20`).
- [SURPLUS] `apidoc` (2 constants) and `response` (1 type) exist only because domain folders may not import each other. They stay, but are named for what they share.
- [SPECIAL] `tasks.hcl:28,33`, `users.hcl:21,26` and `exchange_history.hcl:44` use `timestamp`, where every other table uses `timestamptz`. They are read models, so they can be rebuilt.
- [HIDDEN] `ListExchanges ORDER BY id DESC` (`exchange.sql:30`) relies on bigserial order matching event order. Order by what the event says.
- [SPECIAL, bug] `integration-test/helpers_test.go:170-172` returns on the first connection error ✔, so the health check never retries "connection refused".

### O. The generator follows the app, and loses what only multi-place edits needed

After A, G, I and J, a new domain needs one new file in `internal/app` plus one list entry, and the
`resolver.go` embed. gqlgen `autobind` removes the `gqlgen.yml` edit, with `Conversion` pinned
explicitly. With one file per operation per transport (the rule `usecase/` already follows), new-usecase
creates files instead of editing 13 places. Three edits remain: the `register.go` line, the REST
`NewRoutes` line, and the proto `rpc` line, because a proto service can't be split across files.

The generator agent estimates it shrinks from 4.2k to about 1–1.2k non-test lines.

- [HIDDEN] The generator edits 9 shared files at 15 anchors (`entity/event.go`, `errors.go`, `contracts.go`, `buses.go` ×2, the REST and gRPC `v1/router.go`, `resolver.go` ×7, `gqlerr.go`, `gqlgen.yml`). Its insert templates even depend on the host function's local variable names (`layer_usecase.go:22-27`, `layer_rest.go:20`, `layer_grpc.go:22`, `layer_graphql.go:40`).
- [SURPLUS] `edit/golists.go` (341 lines) and `edit/yaml.go` (86 lines) serve 6 anchors. Five locators have a single caller each.
- [NEEDLESS] The `templates.Usecase` fields are filled as a side effect of edit closures and read by later renders (`templates.go:78-95`). Reordering the edits silently renders `""`.
- [HIDDEN] `execute.go:15-26` `regenerated()` repeats the Makefile's regenerator list and every output path. The Makefile should own the list, and the undo should come from `git status` after regenerating.
- [HIDDEN] `names/names.go:28-36` hand-keeps the app's package names; `go list` can supply them.
- [HIDDEN] Each transport's controller path is written in 8 places (`rules/r2_targets.go:22-31`, `rules/r1_name.go:24-30`, `layer_rest|grpc|graphql.go`, `usecase_rest|grpc|graphql.go`).
- [HIDDEN] `rules/r2_routes.go:50` finds routes by scanning Go at fixed depths. It can't see an operation built by a helper. It should read `docs/openapi/v1.json`.
- [SPECIAL] Per-domain exceptions:
  - the `UserID` rule (`r2_targets.go:84`);
  - exchange's `History` field (`usecase_message.go:20-33`), which goes once the hand-written domains are uniform. This is an internal rename only.
  - Kept: user serves `AuthService` from `auth.proto`, because renaming it would break gRPC clients.
- [SPECIAL] Swallowed errors:
  - `layer.go:104` and `usecase.go:178` (format errors deferred to R5);
  - `r1_name.go:87,158`;
  - `r2_routes.go:64,69`: a broken file registers no routes, so the route check passes;
  - `r2_targets.go:72,121,126`, `usecase_grpc.go:90,96`, `rules/repo.go:62-68`;
  - `domain.go:63-68` and `usecase.go:89-94` continue planning with an empty module.
- [SPECIAL, bug] `rules/repo.go:84-86` keeps a porcelain rename line as the single path `"old -> new"`, and never unquotes quoted paths.
- [SPECIAL] `edit/golists.go:217-231` `AfterLastStmt` accepts N≥1 matches, though R4 promises exactly one.
- [SURPLUS] Exported only for tests: `rules.Creatable/Anchors/Valid/DomainTransports/ErrClean`, `templates.Render/ErrNoTemplates`, `plan.ErrWrite`, `edit.Field`. The `tmpl fs.FS` parameter is always `templates.FS()`. `planner.used` is production state that only one test reads.
- [NEEDLESS] Rule sentinels are spread over 4 packages, `rules.Repo` owns I/O, and the helpers are duplicated (`layer.go:130` = `rules/repo.go:55`, `r2_targets.go:118-153` ≈ `r1_name.go:249`).
- Considered and rejected: generating from a compiled placeholder domain inside the app. That would be a second fake domain shipped in the app, which is surplus. The templates stay the generator's reference, kept in step with `task` by gen-e2e.

### P. Comments (chosen)

Remove the 247 `// X -.` placeholder doc comments ✔ (the lint config does not require them), plus every
comment that restates the code. Keep comments about facts outside the code. Remove comments that
explain workarounds this refactor removes.

### Q. Tests (chosen, done last)

From the test review: about 130 of 554 functions, plus about 110 subtests.

- **Remove: tests that only repeat the code.**
  - Pass-through query tests that assert the mock's return value (task, exchange, user, passkey `query_test.go`).
  - `EventTypes` re-listings in 4 aggregate tests.
  - Field-copy gRPC mapper tests.
  - `deliver_email_test.go:43`.
  - `RoleNames(LocalRoles())` used as the expected value.
- **Remove: tests that can't fail.**
  - Getter and constant tests: projector `Name()/Tables()` and importer `Domain()` ×8. These are replaced by one pin of the stored names in `internal/app`, because those names are database keys.
  - `config` tests that repeat an `envDefault` tag.
  - Tests that only exercise their own fixture (`aggregate_internal_test.go:93`, `sso_test.go:237`).
- **Rewrite: tests tied to internals.**
  - Exact log text (`cqrs/steps_test.go:61`) and private fields.
  - The SHA-256 challenge key (`login_test.go:84`, `ceremony_test.go:338`) and `claims[2]` order.
  - Message substrings in the gRPC auth tests.
  - Generator file counts and exact wording.
- **Consolidate: duplicates.**
  - Aggregate rules re-tested in handlers.
  - Role filtering re-tested in each middleware.
  - Importer skip-on-conflict ×4, which goes to one test of the loop.
  - The `gqlerr` table re-checked in resolver tests.
  - Integration CRUD repeated per transport: one smoke test per transport remains.
- **Fix: tests that are wrong.**
  - `runner_test.go:277,331` sleep for negative assertions and check nothing. Use counters.
  - `cqrs_user_test.go:56` sleeps 3 s to prove no second email.
  - `eventstore_test.go:160` waits 300 ms for a lock, so it can pass falsely.
- **Generator goldens: kept** (decided). They restate the templates rendered for `widget`, but they are the only check of the generator's output that needs no Docker; gen-e2e stays the build/lint/test check. Golden assertions on file counts and exact wording are rewritten as above.

---

## Decisions taken when the plan was approved (defaults stated, the user may overrule)

- K: yes. L: keep the backfill, isolated. Goldens: kept.
- Task ownership keeps today's answers (403 on read/update, 404 on delete): it is public API.
- The 11 controller structs that hold the buses and a logger stay: they are the receivers of the
  operations, and passing the same two values to every operation function would be no simpler.
- `tools/gen` templates follow the app in one step (O), not after every task: they are rewritten
  there anyway. Until then `make gen-check` stays green and `make gen-e2e` is red on this branch.

## Special cases that stay (the system really needs them)

- The retry step on `ErrConcurrencyConflict`: optimistic concurrency.
- The wait step's polling (20 ms): commands are read-your-writes. LISTEN would add a held connection in the app for a small gain.
- Passkey login's own transaction: burning the challenge and recording the use commit together.
- Integration tests polling: the projections and the queue really are eventually consistent.
- `graphql/resolver.go` listing domains: gqlgen requires one root type.
- User's gRPC `AuthService` name and every REST/gRPC/GraphQL path and status, except the bug fixes listed: they are the public API.
- The `*Imported` events: events are forever.
- The email envelope in `entity`: both ends (notify, the worker controller) are this codebase, and `entity` is the only package both may import.

## Behaviour that changes (bugs the refactor fixes)

Everything else a caller or an operator sees stays as it was. As merged:

API
- gRPC checks task and sign-up input in the domain, as REST does, and answers `InvalidArgument`.
- Every list pages by one rule (`entity.Page`): limit 0–100, 0 meaning 10. The REST task list, which had no maximum, and gRPC lists are capped at 100.
- The OpenAPI document lists 409, 503 and 501 on every command, oidc-status's errors, and 502 on convert-currency; `docs/openapi/v1.json` and the web types changed with it.
- Any failure of the rate provider (unreachable, timed out, non-2xx, an error result, an unreadable body) answers 502, as an SSO outage does; it was 500 for most of them.
- An SSO provider outage answers 502 at every step of a sign-in (discovery, the code exchange, the key fetch), not only the first lookup: after discovery it was a 500, or a 401 for a token endpoint answering 5xx and for keys that could not be fetched. A step gives up on the provider after 5 s (`sso.Timeout`); it had no timeout. The HTTP write deadline covers that wait as it does the rate's.
- An amount is parsed once, in the domain: gRPC's message for an unparsable amount is the entity's.
- A read of an ID other than a lowercase UUID, as the app writes them, answers not found instead of 500, as a write to it already did.
- Task lists page in one order when two tasks share a timestamp.
- A sign-in whose user read fails answers 500 and is logged, instead of 401.
- REST paths no operation serves answer 404 instead of 401: each operation declares whether it needs a token.
- Only HS256 tokens are accepted.
- With `DOCS_ENABLED=false`, GraphQL refuses introspection and gRPC serves no reflection, as REST serves no document.

Operation
- gRPC recovers from panics, and logs and measures every call; GraphQL logs a panic with its stack.
- A stuck projector holds up only the writes whose events it handles.
- Logs are `log/slog` JSON on stdout, the line saying why a binary stopped included. Spans: the per-repository spans are gone; the message, SQL and HTTP spans remain, with no email in them. The REST correlation ID is stored even with tracing off, as gRPC's was.
- Metrics count whatever `METRICS_ENABLED` says; only `/metrics` reads it.
- At startup the database is waited for until the binary is told to stop. The worker starts without `HTTP_PORT` or `JWT_SECRET`, and `HTTP_PORT` defaults to 8080 as `GRPC_PORT` does to 8081. An `OIDC_ROLE_MAP` role the app does not have stops the app at startup.
- On SIGTERM each server gets 3 s to drain, then drops what is left with a warning and stops cleanly: gRPC could wait forever on an open stream, and a slow HTTP request made the exit code 1.
- A queue handler is stopped at its visibility timeout, and one registered after `Run` is refused; the demo queue is gone.

## Order of work (each step ends green: `make format`, `make linter-golangci`, `make test`; `make gen-check` + `make gen-e2e` whenever a mirrored file changes; docs updated in the same step)

1. K (slog), if approved. Then N's integration health-check fix, so the integration suite is a trustworthy net.
2. A (error table) → D (messages) → B (domain rules, paging, schema). The templates follow in the same commits.
3. C (write model) → F (ports for adapters and sessions).
4. E (projections and codec) → I (decorators gone) → J (contracts per domain).
5. G + H (composition root, bootstrap, lifecycle, config, auth) → M (worker/queue).
6. L (backfill), kept and isolated.
7. O (generator simplification).
8. P (comments) → Q (tests) → a final sweep by fresh reviewers for remaining special cases and links, repeated until nothing is worth fixing. Then `make compose-up-integration-test` to prove behaviour.

Commits are Conventional: `refactor:` for structure, and `fix:` for the listed bugs, which means the
merge releases a patch or minor version.

---

## Sweep 1 (task 35): what fresh reviewers still found, grouped by fix

Three read-only reviewers (entity+usecase+pkg, repo+app+config+cmd, controllers) at c446f4d.

### S1. One metrics registry and one tracing switch, read at the composition root
`METRICS_ENABLED` is read four times and `cqrs.Metrics` registers whatever it says; `TRACING_ENABLED` gates
HTTP tracing while gRPC, pgx and the buses trace unconditionally (so a gRPC write stores a correlation
ID a REST write would not). Both transports copy a reuse-on-AlreadyRegistered helper only tests reach.
Fix: bootstrap builds one `prometheus.Registerer` (a throwaway registry when metrics are off) and passes
it to the buses, both transports' metrics and the lag collector; only `/metrics` reads the flag; HTTP
tracing is always installed (the provider is already a no-op when off).

### S2. Startup and lifecycle facts in one place
- `pkg/postgres.New` takes no ctx (a startup wait of ~20 s ignores the shutdown signal), sleeps after its
  last attempt, keeps test-only options and `_default*` constants: `New(ctx, url, poolSize)` retrying
  until ctx ends.
- The HTTP WriteTimeout margin covers the projection wait but not the rate API's 5 s client timeout.
- `HTTP_PORT` and `JWT_SECRET` are `required` for the worker, which reads neither.
- `importer func(*deps)` takes the whole deps though every importer uses only the pool; the pool is
  opened in two places.
- `pkg/worker`'s Handle-before-Run rule is a comment over an unlocked map.
- An `OIDC_ROLE_MAP` naming no app role is dropped silently on every sign-in instead of failing startup.
- `make schema-apply` still appends `?sslmode=disable` to a URL that may have a query.
- `queue_messages` still uses `timestamp`/`varchar(255)`, and `EnqueuedAt`/`VisibleAt` are read by nobody.
- The web client's `/login` and `/sso/callback` routes are hard-coded in two adapters.
- Dead surface: `jwt.ParseToken`, `ErrUnexpectedSigningMethod` (and a hand-written HMAC check where
  `WithValidMethods` pins HS256), `smtp.Timeout`.

### S3. Read access is a domain rule, decided once
Who may read a task, a passkey or a conversion is decided in the read models, three ways (a Go check
answering Forbidden, a Go check answering NotFound, an SQL filter), apart from the aggregates' write-side
rules. Fix: read models return a record by ID; an entity method (`ReadableBy`) used by the query handler
decides, keeping today's answers.

### S4. The write side's remaining loose ends
- Both passkey handlers load by hand (stream naming, `ErrStreamNotFound`); `StreamOf` invents an empty
  stream for an undecodable ID. Fix: `aggregate.Load`, and `StreamOf` answering the aggregate's not-found.
- `aggregate.Now` still truncates to Postgres's precision inside the use cases; nothing compares an
  event's time with its row's.
- `Wait` takes `[]repo.Subscriber` (event handlers fit the type), rescans `Handles()` per command, and
  keeps the `WrittenAt() == 0` convention: pass a map from event type to projector names built once.
- Seven sentinel messages restate their constants by hand (and the page one is wrong: 0 is accepted).
- The four projectors repeat a 25-line shell and export `Project` only for tests: one
  `projection.NewProjector(name, tables, handles, pg, project)`.
- `projection.Reader` has one implementation; `LastPosition` is production SQL only tests call;
  memstore writes row fields nothing reads.

### S5. Transports: the last per-transport copies
- The amount is parsed in REST and gRPC, each with its own error (two messages for one rule on gRPC):
  parse once in the domain.
- The schema-hint pin misses the task-status enum, the passkey name query parameter and GraphQL's
  `limit` defaults.
- The REST passkey folder alone wraps each send in a one-caller helper.
- GraphQL logs a panic without its stack; REST and gRPC log `panic` and `stack`.
- The HTTP-wide middleware (log, trace, metrics, recovery) lives in `restapi/middleware` though it wraps
  `/graphql`, `/healthz` and `/metrics`: its own package; `restapi` keeps `Auth`.
- `oidc-status` lists no `Errors` (OpenAPI shows `default`); deletes write 204 three ways; "an empty
  status means any" is written in REST and gRPC; a rate-API outage answers 500 where an SSO outage
  answers 502 (`KindUpstream`).
- Docs and comments that now contradict the code (cqrs.md's `_` subscriber rule, "upserts",
  `projection.Names`'s doc, the codec/upcaster and trace-span comments, step 4's missing `Handles`).

Kept after the sweep: `internal/repo` adapters importing `config` for their sections (config is plain
structs; mirror structs were the problem G removed). Tests: adding handler tests for gRPC exchange/user
is out of the user's chosen "Tests" extra (remove/fix), except where a fix above needs one.

## Sweep 2 (task 35): what fresh reviewers found after S1–S5

Three read-only reviewers (domain and data; transports and wiring; generator and docs) at 3c3d795.

### U1. The generator still copied the task domain from before S3 and S4
It was written on a tree from before them, and the first re-sync looked only at what changed after
it was merged. A generated domain decided read access in its read model, built its projector by
hand, and had a `StreamOf` that `aggregate.Change` no longer accepts, which gen-e2e could not see
because the stub handlers never call it. Fix: the templates copy the task files again, and the
generated `command_test.go` names `aggregate.Change`, so gen-e2e fails when the aggregate stops
fitting it.

### U2. Reads and writes answer the same about an ID
A read of an ID that is not a UUID answered 500 (Postgres refuses it for a `uuid` column), and one
in another form (upper case, `urn:uuid:`) found a task that a write of the same ID answered 404 for.
Reads by ID now accept only a lowercase UUID, as the write side's stream names do. Also: task lists
page with an `id` tiebreaker; a failed user read at sign-in is an error, not 401; any rate-provider
failure is `ErrRatesUnavailable`.

### U3. Lifecycle
gRPC's drain had no bound and an HTTP drain that ran out made the exit code 1: both are bounded,
then forced, and a signalled stop is clean. The stop line was text on stderr. The worker built an
event store it never used. `projection.Runner.Add` kept "before Run" as a comment: the runner takes
its subscribers in `New`.

### U4. One switch per API reference
`DOCS_ENABLED` hid REST's document and GraphiQL but not GraphQL introspection or gRPC reflection.

### U5. Hidden links and surplus
- Each transport mapped kinds through an array indexed by `entity.Kind`, which a new kind would
  overrun at run time: switches, so `exhaustive` fails the lint instead.
- Named empty input types, `readBack`, `input.go`, a stale `_tracerName`, `ErrNoHTTPPort`,
  `CeremonyPurpose`, `Message.Queue`, `PasskeyLogin.PasskeyID`; `TxFrom` and `projection.Names`
  exported for nobody; a mailer test that asserted nothing it recorded.
- Generator: R5's format check no created file could reach, `Import`'s alias, `Insertion.Path` and
  `templates.FS`, which only tests used.
- Docs that named code that changed (`architecture.md`'s read example and controller exceptions,
  CLAUDE.md's file lists and names).

Kept after sweep 2:
- `planner.set` in the generator: the planner's tests key templates by set across several plans.
- The worker's metrics registry, which nothing serves: a worker `/metrics` endpoint is a feature.
- A provider answer that succeeds with no rates is `ErrUnsupportedCurrency` (400).
- GraphQL's refused introspection carries no `extensions.code`: gqlgen's own presenter answers it.
- Sign-in timing: an unknown email answers before the password hash is compared, so response time
  can tell which emails exist. It predates the refactor and is a security change, not an untangling
  one: reported to the user rather than changed here.

## Sweep 3 (task 35): the last pass

Two read-only reviewers at 3c5899f: one over everything sweep 2 changed, one over the whole Go code
with a high bar (bugs, special cases, hidden or needless links, real surplus only).

- Sweep 2's fixes held. What it left: four comments it made stale, CLAUDE.md saying a `default`
  satisfies `exhaustive` (this config does not let it), and the task read model's new test with no
  generated twin. Fixed.
- An SSO provider outage answered 502 only at the first lookup; after it, the code exchange and the
  key fetch answered 500 or 401, and every provider call ran without a timeout. Fixed as the rate
  provider was: every provider failure is `ErrOIDCUnavailable`, a 4xx from the token endpoint stays
  the caller's bad ceremony, and the calls are bounded.

Kept after sweep 3:
- Releasing a claim (`entity.Claim.Release`, the stores' release path, its contract test), which no
  domain uses yet: it is the other half of the claim contract, both stores pass it, and without it
  the first rename or account deletion would have to change the store in three places.
- `ConvertCurrency` checking its input in the handler (`entity.NewConversion`), not `Validate()`:
  the amount is parsed once and every transport gets the same answer.
