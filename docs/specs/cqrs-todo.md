# Tasks: full CQRS with event sourcing

Plan: [cqrs-plan.md](cqrs-plan.md) · Spec: [cqrs.md](cqrs.md)

**Gates used below:**
- **G1:** `make format`, `make linter-golangci`, `make test`.
- **G2:** G1, then `make compose-up-integration-test` (stop `make dev` first).
- **G3:** G2, then `make openapi`, `make web-api` and `make web-graphql` give no diff, and `make web-check` passes.

**Sizes:** S = 1–2 files, M = 3–5, L = 6–8. Where a task lists more than 5 files, they are
one-message files of the same shape, or a mechanical signature change.

---

## Phase 1: Foundation

### 1. Command and query buses
`pkg/cqrs`: the `Command`, `Query` and `Handler` types, `Written`, and the `Commands` and `Queries`
buses with `HandleCommand`, `HandleQuery`, `SendCommand` and `SendQuery`, as in the spec.
- [x] A registered handler gets the message and its result comes back typed. A second handler for
      the same type panics. An unregistered type is `ErrNoHandler`.
- [x] Steps run in the order given, around the handler, and each sees the message name.
- [x] `pkg/cqrs` imports nothing from `internal/`.

**Verify:** G1. **Depends on:** none. **Files:** `pkg/cqrs/cqrs.go`, `bus.go`, `bus_test.go`. **Size:** M

### 2. Trace, log and metrics steps
Steps that every message gets, replacing the use-case `tracing.go` decorators.
- [x] Trace: one span per message, named `command <Name>` or `query <Name>`, marked as failed on error.
- [x] Log: name, duration, outcome, and the wrapped error on failure.
- [x] Metrics: `cqrs_messages_total{kind,name,outcome}` and `cqrs_message_duration_seconds`.

**Verify:** G1. **Depends on:** 1. **Files:** `pkg/cqrs/trace.go`, `log.go`, `metrics.go`, `steps_test.go`. **Size:** M

### 3. Transaction carried in the context
`pkg/postgres` gets `InTx(ctx, fn)` and `Executor(ctx)`. `Executor` returns the transaction in the
context if there is one, and the pool otherwise. Every repository will use it.
- [x] `InTx` commits when `fn` returns nil and rolls back on error or panic.
- [x] Nested `InTx` joins the outer transaction instead of opening a second one.

**Verify:** G1. The real-database path is covered by the suite in task 8. **Depends on:** none.
**Files:** `pkg/postgres/tx.go`, `tx_test.go`. **Size:** S

### 4. Event-sourcing base types
- [x] `entity.AggregateBase` holds the ID, the loaded version and the pending events; `record`
      applies an event and queues it.
- [x] `entity.Event`, `RecordedEvent` (position, stream, version, metadata) and `Metadata` (user,
      correlation, causation), and an `EventTypes()` registry that starts empty.
- [x] Sentinels `ErrConcurrencyConflict` and `ErrReadModelBehind` in `errors.go`.

**Verify:** G1. **Depends on:** none.
**Files:** `internal/entity/aggregate.go`, `event.go`, `errors.go`, `aggregate_test.go`. **Size:** M

### 5. Event-store tables and queries
Four tables as in the spec, and the sqlc queries the store needs.
- [x] `events`, `snapshots`, `claims` and `projection_checkpoints` exist in the HCL. `make sqlc`
      renders them, and the app applies them on start.
- [x] Queries cover: append, read a stream after a version, read all after a position, the stream
      version, snapshot get and put, claim insert, delete and owner, checkpoint get, conditional
      put and reset, and the lag.
- [x] `sqlc.yaml` sets `emit_interface: true`, for the projector tests.

**Verify:** G1, then `make sqlc` leaves the tree clean on a second run. **Depends on:** none.
**Files:** `internal/app/schema/events.hcl`, `snapshots.hcl`, `claims.hcl`,
`projection_checkpoints.hcl`, `internal/repo/persistent/queries/eventstore.sql`, `sqlc.yaml`
(and the generated files). **Size:** L

### 6. EventStore contract, codec and upcasters, repomock
- [x] `repo.EventStore`: `Load`, `Save` (with claims and guard rows), and claim owner lookup.
      `repo.Subscriber`: `Name`, `Handle(ctx, RecordedEvent)`.
- [x] The codec turns events into `(type, schema version, payload)` and back through
      `EventTypes()`. Upcasters map `(type, from)` to the next version. A test proves a version 0
      payload loads through a fake upcaster.
- [x] `make mock` also writes `internal/repo/repomock`. The `usecase_test` mocks stay for now.

**Verify:** G1, then `make mock` leaves the tree clean. **Depends on:** 4.
**Files:** `internal/repo/contracts.go`, `internal/repo/persistent/eventstore/codec.go`, `upcast.go`,
`codec_test.go`, `Makefile`. **Size:** M

### 7. Contract suite and in-memory store
`internal/repo/eventstoretest.Run(t, newStore)` holds one set of tests that every `EventStore` must
pass.
- [x] The suite covers:
  - append and load;
  - two writers at the same version (`ErrConcurrencyConflict`);
  - a taken claim and a released claim;
  - a snapshot load equal to the full replay;
  - an upcast on load.
- [x] `internal/repo/memstore` passes the suite in `make test`.

**Verify:** G1. **Depends on:** 6.
**Files:** `internal/repo/eventstoretest/suite.go`, `internal/repo/memstore/memstore.go`,
`memstore_test.go`. **Size:** M

### 8. Postgres event store
`Save` in one transaction: lock, version check, events, claims and guard rows, snapshot, NOTIFY.
`Load`: latest snapshot, then later events, upcast, apply.
- [x] It passes the contract suite against the real database: `integration-test/eventstore_test.go`
      runs `eventstoretest.Run` through `PG_URL`.
- [x] Positions commit in order: a write held open while a later one commits is still read by a
      reader tailing by position. The test fails with the append lock removed.
- [x] A snapshot is written at each multiple of `EVENTSTORE_SNAPSHOT_EVERY`. A snapshot with an old
      schema version is ignored.

**Verify:** G2. **Depends on:** 3, 5, 7.
**Files:** `internal/repo/persistent/eventstore/eventstore.go`, `snapshot.go`, `tracing.go`,
`integration-test/eventstore_test.go`. **Size:** M

### Checkpoint A: the event store
- [x] The contract suite passes on memstore and on Postgres; G2 is green.

### 9. Projection runner
One goroutine per subscriber in the worker.
- [x] It takes the subscriber's advisory lock, reads a batch after the checkpoint, and applies each
      event plus a conditional checkpoint update in one transaction. Events the subscriber doesn't
      handle still move the checkpoint.
- [x] It wakes on `LISTEN events`, and polls as a fallback. A rebuild (reset to 0 and empty tables)
      makes the running runner reload and replay.
- [x] `projection_lag_events{projector}` is on `/metrics`. An integration test runs a counting
      subscriber against the real database through append, catch-up and reset.

**Verify:** G2. **Depends on:** 8.
**Files:** `internal/repo/persistent/projection/runner.go`, `checkpoint.go`, `rebuild.go`,
`runner_test.go`, `integration-test/projection_test.go`. **Size:** M

### 10. App steps: wait, retry, authorize, validate
`internal/usecase/behavior`.
- [x] Wait: after a command, it polls the named projectors' checkpoints until each reaches
      `Written.Position`, and returns `ErrReadModelBehind` past the timeout. With no projectors
      named, it returns at once.
- [x] Retry: on `ErrConcurrencyConflict`, it runs the handler again, three attempts in all.
- [x] Authorize: it checks `RequiredRole()` against the message's caller (`ErrAdminRequired`).
      Validate: it calls `Validate()` when the message has one.

**Verify:** G1. **Depends on:** 1, 4.
**Files:** `internal/usecase/behavior/wait.go`, `retry.go`, `authorize.go`, `validate.go`,
`behavior_test.go`. **Size:** M

### 11. Shared mapping of the two new sentinels
One shared package per transport maps the two failures any command can produce.
- [x] `ErrReadModelBehind` maps to REST 503, gRPC `Unavailable`, GraphQL `UNAVAILABLE`.
      `ErrConcurrencyConflict` maps to 409, `Aborted`, `CONFLICT`. This waits on open question 1;
      the default is used if it's unanswered.
- [x] `docs/openapi/v1.json` doesn't change.

**Verify:** G1, then `make openapi` gives no diff. **Depends on:** 4.
**Files:** `internal/controller/restapi/v1/apierr/apierr.go`, `internal/controller/grpc/v1/rpcerr/rpcerr.go`,
`internal/controller/graphql/gqlerr/gqlerr.go`, and the tests beside them. **Size:** M

### 12. Config: the four env vars
- [x] `CQRS_CONSISTENCY_TIMEOUT`, `EVENTSTORE_SNAPSHOT_EVERY`, `PROJECTION_POLL_INTERVAL` and
      `PROJECTION_BATCH_SIZE` have defaults, appear in `.env.example`, and are documented.
- [x] `docker-compose.yml` passes them to the app and the worker only if they need it; the defaults
      should make that unnecessary.

**Verify:** G1. **Depends on:** none.
**Files:** `config/config.go`, `.env.example`, `docs/configuration.md`. **Size:** S

### 13. Wiring: buses, worker runner, rebuild command
- [x] `internal/app/buses.go` builds both buses with the steps in the spec's order. `initUseCases`
      returns them next to today's use cases. No domain registers yet.
- [x] The worker starts the runner with an empty subscriber list, and stops it on shutdown.
- [x] `make projection-rebuild name=<projector>` runs `worker -rebuild <name>`. An unknown name
      fails and lists the known ones.

**Verify:** G2: nothing behaves differently. **Depends on:** 2, 9, 10, 12.
**Files:** `internal/app/buses.go`, `internal/app/app.go`, `internal/app/worker.go`,
`cmd/worker/main.go`, `Makefile`. **Size:** M

### 14. Backfill framework
- [x] An `Importer` (`Domain()`, `Import(ctx)`) runs once per domain at app start, under an advisory
      lock, in one transaction with its marker `_backfill/<domain>`.
- [x] With no importers registered, app start does nothing new.

**Verify:** G2. **Depends on:** 8.
**Files:** `internal/repo/persistent/eventstore/backfill.go`, `internal/repo/contracts.go`,
`internal/app/app.go`. **Size:** S

### Checkpoint B: foundation
- [x] `make pre-commit` passes, G3 is green, and behaviour is unchanged.
- [ ] **Your review** of the bus, event store and runner APIs.

---

## Phase 2: Task, the pattern

### 15. TaskAggregate and its events
- [x] `TaskAggregate` has `NewTaskAggregate`, `Update`, `Transition` and `Delete`. Each records
      `TaskCreated`, `TaskUpdated`, `TaskTransitioned` or `TaskDeleted`, and `TaskImported` restores
      the whole state. The rules from `entity.Task` move here.
- [x] Given/when/then tests cover every method and every rule error. `entity.Task`, the read shape,
      doesn't change.
- [x] The task events are registered in `EventTypes()`.

**Verify:** G1, then `make openapi` gives no diff. **Depends on:** 4.
**Files:** `internal/entity/task.go`, `task_events.go`, `task_test.go`, `event.go`. **Size:** M

### 16. Task read model and projector
- [x] `repo.TaskReadModel` has `Get`, `List` and `ListAll`. The old `TaskRepo` stays until task 21.
- [x] The projector upserts on `TaskCreated` and `TaskImported`, updates on `TaskUpdated` and
      `TaskTransitioned`, and deletes on `TaskDeleted`. It's tested against a mocked `Querier`.
- [x] `tasks` loses its foreign key to `users`.

**Verify:** G1, then `make sqlc` gives a clean tree. **Depends on:** 5, 15.
**Files:** `internal/repo/contracts.go`, `internal/repo/persistent/task/projector.go`,
`projector_test.go`, `internal/repo/persistent/queries/task.sql`, `internal/app/schema/tasks.hcl`.
**Size:** M

### 17. CreateTask and UpdateTask, and the load–change–save helper
- [x] The generic helper loads an aggregate, runs a change and saves it, returning `cqrs.Written`.
      It lives next to the `EventStore` contract.
- [x] `CreateTask` and `UpdateTask` follow the spec's code style, one file each.
- [x] memstore tests: given these events, the command appends these events or returns today's
      error (`ErrTaskNotFound`, `ErrTaskForbidden`).

**Verify:** G1. **Depends on:** 7, 15.
**Files:** the helper, `internal/usecase/task/command/create_task.go`, `create_task_test.go`,
`update_task.go`, `update_task_test.go`. **Size:** M

### 18. TransitionTask and DeleteTask
- [x] `TransitionTask` returns `ErrInvalidTransition` exactly where today's use case does.
- [x] `DeleteTask` makes the stream load as not found afterwards.
- [x] Both have memstore tests.

**Verify:** G1. **Depends on:** 17.
**Files:** `internal/usecase/task/command/transition_task.go`, `transition_task_test.go`,
`delete_task.go`, `delete_task_test.go`. **Size:** M

### 19. GetTask, ListTasks, ListAllTasks
- [x] Each reads only `repo.TaskReadModel`.
- [x] `ListAllTasks` declares `RequiredRole() = RoleAdmin` and carries its caller.
- [x] `repomock` tests give each query today's results and today's errors.

**Verify:** G1. **Depends on:** 6, 16.
**Files:** `internal/usecase/task/query/get_task.go`, `list_tasks.go`, `list_all_tasks.go`, and
their three tests. **Size:** L (six small files)

### 20. Task registration, importer and wiring
- [x] `task.Register(cmds, qs, deps)` registers all seven handlers. `buses.go` calls it.
- [x] The worker runs the `task` projector, and the wait step waits for it.
- [x] The task importer turns each `tasks` row into a `TaskImported` event, and is registered with
      the backfill.

**Verify:** G2: the integration tests are still green, with nothing switched yet. **Depends on:**
13, 14, 16, 17, 18, 19.
**Files:** `internal/usecase/task/register.go`, `internal/repo/persistent/task/importer.go`,
`internal/app/buses.go`, `internal/app/worker.go`, `internal/app/app.go`. **Size:** M

### 21. Switch: task writes go through the command bus
- [x] Each method of the old task use case sends the matching command or query. A write sends its
      command, then reads back with the query. REST, gRPC and GraphQL therefore all switch here.
- [x] `TaskRepo`'s write methods and `internal/usecase/task_test.go` are removed; the handler tests
      now cover them.
- [x] Every existing integration test passes unchanged. In the web client, under `make dev`,
      creating, editing, moving and deleting a task updates the list at once.

**Verify:** G3, plus the web client check. **Depends on:** 20.
**Files:** `internal/usecase/task/task.go`, `internal/usecase/task_test.go`,
`internal/repo/persistent/task/task.go`, `internal/repo/contracts.go`, `internal/app/app.go`.
**Size:** M

### 22. REST task on the buses
- [x] `restapi.NewRouter` and `v1.NewRoutes` take the buses. `task.NewRoutes` uses them, and maps the
      shared failures through `apierr`.
- [x] The REST task tests run the real buses with fake handlers, with their assertions unchanged.
      `resttest.NewRouter` builds the buses.

**Verify:** G3. **Depends on:** 11, 21.
**Files:** `internal/controller/restapi/router.go`, `v1/router.go`, `v1/task/controller.go`,
`v1/task/task.go`, `v1/task/fake_test.go`, `resttest/resttest.go`. **Size:** L (signature change)

### 23. gRPC task on the buses
- [x] `grpc/v1/router.go` gets one `NewRoutes(srv, cmds, qs, l)` listing the domains, and
      `grpc/router.go` calls it. The task service uses the buses.
- [x] The gRPC task tests pass, with their assertions unchanged.

**Verify:** G3. **Depends on:** 11, 21.
**Files:** `internal/controller/grpc/router.go`, `v1/router.go`, `v1/task/controller.go`,
`v1/task/task.go`, `v1/task/task_test.go`. **Size:** M

### 24. GraphQL task on the buses
- [x] `graphql.NewHandler` takes the buses, and the task resolvers use them.
- [x] `graphqltest.Post` builds the buses. The GraphQL task tests pass, with their assertions
      unchanged.

**Verify:** G3. **Depends on:** 11, 21.
**Files:** `internal/controller/graphql/handler.go`, `resolver.go`, `task/resolver.go`,
`task/query.go`, `task/mutation.go`, `task/fake_test.go`, `graphqltest/graphqltest.go`.
**Size:** L (signature change)

### 25. Remove the old task use case; CQRS integration tests
- [x] `usecase.Task`, `internal/usecase/task/task.go` and its `tracing.go` are gone, along with the
      task parameter of every router.
- [x] New integration tests cover:
  - read-after-write on REST, gRPC and GraphQL;
  - two concurrent updates of one task, which must end in one success, or a retry that succeeds,
    and never a lost write;
  - `make projection-rebuild name=task` followed by identical reads.

**Verify:** G3. **Depends on:** 22, 23, 24.
**Files:** `internal/usecase/contracts.go`, `internal/usecase/task/task.go`,
`internal/usecase/task/tracing.go`, `internal/app/app.go`, `integration-test/cqrs_task_test.go`.
**Size:** M

### Checkpoint C: the pattern
- [x] `make pre-commit` passes, G3 is green, and tasks in the web client behave exactly as before.
- [ ] **Your review** of the task domain. Every later domain copies it, so changes are cheapest
      here.

---

## Phase 3: User and the welcome email

### 26. UserAggregate and its events
- [x] `UserRegistered` (username, email, password hash), `UserRegisteredViaOIDC` (with issuer and
      subject) and `UserImported` (with identities).
- [x] The aggregate method states which claims a registration needs: `user.email`, `user.username`
      and, for OIDC, `oidc.identity`.
- [x] Tests, and the events registered in `EventTypes()`.

**Verify:** G1. **Depends on:** 4.
**Files:** `internal/entity/user.go`, `user_events.go`, `user_test.go`, `event.go`. **Size:** M

### 27. User read model and projector
- [x] `repo.UserReadModel` has `ByID` and `ByEmail`, plus the identity lookup.
- [x] The projector writes `users` and `user_identities`, upserting on import.
- [x] `user_identities` loses its foreign key.

**Verify:** G1, then `make sqlc` gives a clean tree. **Depends on:** 5, 26.
**Files:** `internal/repo/contracts.go`, `internal/repo/persistent/user/projector.go`,
`projector_test.go`, `internal/app/schema/user_identities.hcl`, `internal/repo/persistent/queries/user.sql`.
**Size:** M

### 28. RegisterUser, LogIn, GetUser
- [x] `RegisterUser` reserves the email and username claims in the append transaction. A taken
      claim returns today's `ErrUserAlreadyExists`.
- [x] `LogIn` reads the read model and returns today's token or today's error. `GetUser` reads the
      read model.
- [x] memstore and repomock tests.

**Verify:** G1. **Depends on:** 17, 27.
**Files:** `internal/usecase/user/command/register_user.go`, its test,
`internal/usecase/user/query/log_in.go`, `get_user.go`, and their tests. **Size:** L (six small files)

### 29. SendWelcomeEmail event handler
- [x] On `UserRegistered`, it calls `repo.UserNotifier.Welcome`. The queue's `Send` writes through
      `Executor(ctx)`, so the enqueue joins the runner's transaction.
- [x] It ignores `UserImported` and `UserRegisteredViaOIDC`.
- [x] Unit test; the worker runs it as a subscriber.

**Verify:** G1. **Depends on:** 9, 26.
**Files:** `internal/usecase/user/event/send_welcome_email.go`, its test,
`internal/repo/persistent/queue/queue.go`, `internal/app/worker.go`. **Size:** M

### 30. User registration, importer and switch
- [x] `user.Register` and the `user` projector are wired up. The importer turns `users` and
      `user_identities` rows into `UserImported` events and claims.
- [x] The old user use case sends the messages. The notifier call leaves `Register`, along with
      `UserRepo`'s write methods and the old user tests.
- [x] The integration tests pass unchanged. `mail_test.go` still sees exactly one welcome email per
      registration.

**Verify:** G3. **Depends on:** 28, 29.
**Files:** `internal/usecase/user/register.go`, `internal/repo/persistent/user/importer.go`,
`internal/usecase/user/user.go`, `internal/app/buses.go`, `internal/app/app.go`. **Size:** M

### 31. REST and gRPC user on the buses
- [x] The user REST operations and the gRPC auth service use the buses, and their tests pass
      unchanged.

**Verify:** G3. **Depends on:** 22, 23, 30.
**Files:** `internal/controller/restapi/v1/user/controller.go`, `user.go`, `fake_test.go`,
`internal/controller/grpc/v1/user/controller.go`, `user.go`, `internal/controller/grpc/v1/router.go`.
**Size:** L

### 32. Remove the old user use case
- [x] `usecase.User`, the user `user.go` and `tracing.go`, and the old user tests
      (`user_test.go`, `user_welcome_test.go`) are gone, along with the user router parameter.
- [x] An integration test checks read-after-write for register followed by an immediate login.

**Verify:** G3. **Depends on:** 31.
**Files:** `internal/usecase/contracts.go`, `internal/usecase/user/user.go`, `tracing.go`,
`internal/app/app.go`, `integration-test/cqrs_user_test.go`. **Size:** M

### Checkpoint D: user
- [x] G3 is green, and each registration sends exactly one welcome email.

---

## Phase 4: Exchange

### 33. ConversionAggregate and its events
- [x] `CurrencyConverted` (amounts as decimal strings, rate, rate date) and `ConversionImported`.
      Tests, and the events registered.

**Verify:** G1. **Depends on:** 4.
**Files:** `internal/entity/exchange.go`, `exchange_events.go`, `exchange_test.go`, `event.go`.
**Size:** M

### 34. Exchange read model and projector
- [x] `exchange_history` gains `conversion_id` and loses its foreign key. The projector inserts on
      both events, upserting on import.
- [x] `repo.ExchangeReadModel` has `History` and `Conversion`.

**Verify:** G1, then `make sqlc` gives a clean tree. **Depends on:** 5, 33.
**Files:** `internal/app/schema/exchange_history.hcl`, `internal/repo/persistent/queries/exchange.sql`,
`internal/repo/persistent/exchange/projector.go`, `projector_test.go`, `internal/repo/contracts.go`.
**Size:** M

### 35. ConvertCurrency, GetExchangeHistory, GetConversion
- [x] `ConvertCurrency` fetches the rate through `repo.ExchangeWebAPI`, then saves a new
      `ConversionAggregate`. An unsupported currency is today's error.
- [x] Both queries read the read model only. memstore and repomock tests.

**Verify:** G1. **Depends on:** 17, 34.
**Files:** `internal/usecase/exchange/command/convert_currency.go`, its test,
`internal/usecase/exchange/query/get_exchange_history.go`, `get_conversion.go`, and their tests.
**Size:** L (six small files)

### 36. Exchange registration, importer and switch
- [x] Registration and the projector are wired up. The importer turns each `exchange_history` row
      into a `ConversionImported` event, and sets `conversion_id` on the row.
- [x] The old exchange use case sends the messages. `ExchangeRepo.Store` and the old exchange tests
      are gone.

**Verify:** G3. **Depends on:** 35.
**Files:** `internal/usecase/exchange/register.go`, `internal/repo/persistent/exchange/importer.go`,
`internal/usecase/exchange/exchange.go`, `internal/app/buses.go`, `internal/app/app.go`. **Size:** M

### 37. REST, gRPC and GraphQL exchange on the buses
- [x] All three exchange controllers use the buses. Their tests pass unchanged.

**Verify:** G3. **Depends on:** 22, 23, 24, 36.
**Files:** `internal/controller/restapi/v1/exchange/exchange.go`, `fake_test.go`,
`internal/controller/grpc/v1/exchange/exchange.go`, `internal/controller/graphql/exchange/query.go`,
`resolver.go`, `fake_test.go`. **Size:** L

### 38. Remove the old exchange use case; update `docs/request-flow.md`
- [x] `usecase.Exchange`, `exchange.go` and its `tracing.go` are gone, along with the router
      parameter.
- [x] `docs/request-flow.md` quotes the new Convert path: controller, bus, handler, aggregate, store,
      projector, query.

**Verify:** G3. **Depends on:** 37.
**Files:** `internal/usecase/contracts.go`, `internal/usecase/exchange/exchange.go`, `tracing.go`,
`internal/app/app.go`, `docs/request-flow.md`. **Size:** M

### Checkpoint E: exchange
- [x] G3 is green.

---

## Phase 5: Passkey

### 39. PasskeyAggregate and its events
- [x] `PasskeyRegistered`, `PasskeyRenamed`, `PasskeyUsed` (credential JSON, used at),
      `PasskeyDeleted` and `PasskeyImported`. The rename and delete owner checks give today's
      `ErrPasskeyNotFound`.
- [x] Re-registering a deleted credential ID behaves as it does today. Tests, and the events
      registered.

**Verify:** G1. **Depends on:** 4.
**Files:** `internal/entity/passkey.go`, `passkey_events.go`, `passkey_test.go`, `event.go`.
**Size:** M

### 40. Passkey read model and projector
- [x] `webauthn_credentials` is written by the projector only, and loses its foreign key.
- [x] `repo.PasskeyReadModel` has `ListByUser` and `Get`. `webauthn_used_challenges` stays a
      write-side guard.

**Verify:** G1, then `make sqlc` gives a clean tree. **Depends on:** 5, 39.
**Files:** `internal/app/schema/webauthn_credentials.hcl`, `internal/repo/persistent/queries/passkey.sql`,
`internal/repo/persistent/passkey/projector.go`, `projector_test.go`, `internal/repo/contracts.go`.
**Size:** M

### 41. FinishPasskeyRegistration and RenamePasskey
- [x] A registration whose stream already exists gives today's `ErrPasskeyExists`. A bad body or
      token gives today's ceremony errors.
- [x] memstore tests.

**Verify:** G1. **Depends on:** 17, 39.
**Files:** `internal/usecase/passkey/command/finish_passkey_registration.go`, its test,
`rename_passkey.go`, its test. **Size:** M

### 42. DeletePasskey and FinishPasskeyLogin
- [x] `FinishPasskeyLogin` saves `PasskeyUsed` and burns the challenge as a guard row in the same
      transaction. A second finish is `ErrPasskeyChallengeUsed`. It returns today's token.
- [x] Expired challenges are still cleaned up as they are today.
- [x] memstore tests.

**Verify:** G1. **Depends on:** 41.
**Files:** `internal/usecase/passkey/command/delete_passkey.go`, its test,
`finish_passkey_login.go`, its test. **Size:** M

### 43. Passkey queries
- [x] `BeginPasskeyRegistration` (it still checks the reauth age, with `ErrPasskeyReauthRequired`),
      `BeginPasskeyLogin`, `ListPasskeys` and `GetPasskey`, read-only.
- [x] repomock tests.

**Verify:** G1. **Depends on:** 40.
**Files:** four query files under `internal/usecase/passkey/query/` and their tests. **Size:** L
(eight small files)

### 44. Passkey registration, importer and switch
- [x] Registration and the projector are wired up. The importer turns `webauthn_credentials` rows
      into `PasskeyImported` events.
- [x] The old passkey use case sends the messages. `PasskeyRepo`'s write methods and the old tests
      are gone.

**Verify:** G3, including `integration-test/passkey_test.go`. **Depends on:** 42, 43.
**Files:** `internal/usecase/passkey/register.go`, `internal/repo/persistent/passkey/importer.go`,
`internal/usecase/passkey/passkey.go`, `internal/app/buses.go`, `internal/app/passkey.go`. **Size:** M

### 45. REST passkey on the buses; remove the old use case
- [x] The passkey controller uses the buses, and `WithPasskeys()` carries no use case.
- [x] `usecase.Passkey`, `passkey.go` and its `tracing.go` are gone.

**Verify:** G3. **Depends on:** 22, 44.
**Files:** `internal/controller/restapi/v1/passkey/controller.go`, `passkey.go`, `passkey_test.go`,
`internal/controller/restapi/passkey.go`, `internal/usecase/contracts.go`. **Size:** M

### Checkpoint F: passkey
- [x] G3 is green, including the passkey integration tests.

---

## Phase 6: OIDC and mail

### 46. FinishOIDCLogin, BeginOIDCLogin, GetOIDCStatus
- [x] `FinishOIDCLogin` looks up the `oidc.identity` claim. For an unknown identity, it saves a
      `UserRegisteredViaOIDC` with the email, username and identity claims, and gives today's
      `ErrAccountConflict` when they're taken. It returns today's token with the mapped roles.
- [x] With no provider configured, both OIDC messages give `ErrOIDCDisabled`, as today.
- [x] Tests.

**Verify:** G1. **Depends on:** 26, 28.
**Files:** `internal/usecase/oidc/command/finish_oidc_login.go`, its test,
`internal/usecase/oidc/query/begin_oidc_login.go`, `get_oidc_status.go`, and their tests.
**Size:** L (six small files)

### 47. OIDC registration, switch, REST on the buses, remove the old use case
- [x] `oidc.Register` is wired up. The OIDC controller uses the buses, and `WithOIDC()` carries no
      use case.
- [x] `usecase.OIDC`, `oidc.go`, its `tracing.go` and the old OIDC tests are gone.
      `integration-test/oidc_test.go` passes against Keycloak.

**Verify:** G3. **Depends on:** 22, 46.
**Files:** `internal/usecase/oidc/register.go`, `internal/controller/restapi/v1/oidc/controller.go`,
`oidc.go`, `internal/controller/restapi/oidc.go`, `internal/app/oidc.go`. **Size:** M

### 48. DeliverEmail; remove `usecase/contracts.go`
- [x] The `DeliverEmail` command and its handler render and send as `Deliver` does today. The
      worker's mail queue handler sends it on the command bus.
- [x] `usecase.Mail`, `usecase/contracts.go` (now empty), the `usecase_test` mocks and their
      `make mock` lines are gone. `mail_test.go` passes.

**Verify:** G3. **Depends on:** 13.
**Files:** `internal/usecase/mail/command/deliver_email.go`, its test,
`internal/controller/worker/v1/mail.go`, `internal/usecase/contracts.go`, `Makefile`. **Size:** M

### Checkpoint G: every domain
- [x] No `internal/usecase/*/tracing.go`, no `usecase/contracts.go`, and no `usecase_test` mocks
      remain. G3 is green.

---

## Phase 7: Finish

### 49. Backfill integration test
- [ ] For each domain, the test:
  1. writes rows straight into the domain's tables;
  2. removes its marker;
  3. runs the importer against the live database;
  4. checks one `*Imported` event per row and the claims, and that the API reads are unchanged;
  5. checks that a second run imports nothing.

**Verify:** G2. **Depends on:** 20, 30, 36, 44.
**Files:** `integration-test/backfill_test.go`. **Size:** S

### 50. Reference docs
- [ ] `docs/cqrs.md` is new, covering:
  - vocabulary and flow;
  - how to add a command, a query, an event, a projector and an event handler;
  - rebuild;
  - the append lock;
  - the behaviour changes.
- [ ] Updated to match the code: `docs/architecture.md` (Rules, Packages), `docs/worker.md`
      (subscribers, rebuild), `docs/development.md` (the worker is required), and the
      Clean Architecture section of `README.md`.

**Verify:** each path and name the docs mention exists (grep). **Depends on:** 48.
**Files:** `docs/cqrs.md`, `docs/architecture.md`, `docs/worker.md`, `docs/development.md`,
`README.md`. **Size:** M

### 51. CLAUDE.md
- [ ] Rewritten for the new shape:
  - the dependency rule;
  - where code goes;
  - the controller layout (buses);
  - the handler contract (step 4 sends a message);
  - "traced decorators" becomes the pipeline steps;
  - mocks become `repomock`;
  - both "Adding…" recipes;
  - the generated files;
  - a docs-table row for `docs/cqrs.md`.

**Verify:** each path and target it names exists. **Depends on:** 50.
**Files:** `CLAUDE.md`. **Size:** S

### 52. Final verification and the PR
- [ ] Backfill on the dev database:
  1. `pg_dump` the dev database to the scratchpad;
  2. record the API reads on `master`;
  3. start the branch;
  4. check that the reads are identical and every row has its `*Imported` event.
- [ ] `make pre-commit`, G3 and every success criterion in the spec hold.
- [ ] When you ask: push the branch and open the PR (subject decided by open question 2), and wait
      for every CI check to be green.

**Verify:** the above. **Depends on:** 49, 51. **Files:** none. **Size:** S

### Checkpoint H: complete
- [ ] Ready for your review and merge.
