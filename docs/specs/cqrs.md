# Spec: full CQRS with event sourcing

Module id: `cqrs` · PR 1 of 2 · Status: **done: merged 2026-11-04 (#45), released in v0.3.0**

## Objective

Move every domain to full CQRS with event sourcing, and change no API contract while doing it.

**Who it is for:** a developer choosing a clean-architecture starter, who puts this repo next to
[jasontaylordev/CleanArchitecture](https://github.com/jasontaylordev/CleanArchitecture) and should
find the Go one ahead. That template stops at commands and queries through MediatR on one
`DbContext`, with domain events dispatched in-process on save; it has no event store, no read
model and no projections. This repo does all of it.

**Agreed in the interview:**

- Full CQRS, nothing skipped: the twelve parts listed under **Success criteria**.
- Event sourcing: events are the stored truth; every aggregate is rebuilt from its events.
- Every domain converts, and the data already in the database is migrated into events.
- One Postgres database. Event store, read models and guards are tables in it.
- Projectors run in the worker. A write waits a bounded time for them, so a read straight after a
  write sees it, as the web client expects today.
- One commit per domain inside the PR, task first as the pattern.

## Vocabulary

| Term | Meaning here |
| --- | --- |
| Command | A request to change state (`CreateTask`). Handled by exactly one command handler. |
| Query | A request to read state (`GetTask`). Handled by exactly one query handler, which reads only read-model tables. |
| Event | A fact that happened (`TaskCreated`). Past tense, immutable, stored forever. |
| Aggregate | The write-side object that takes a command, checks its rules and records events (`TaskAggregate`). |
| Stream | One aggregate's events, in order: `task-<id>`. Its length is the aggregate's version. |
| Position | An event's place in the whole store, increasing across all streams. |
| Projector | Turns events into read-model rows (`tasks` table). |
| Event handler | Reacts to an event with a side effect (send the welcome email). The .NET template's `EventHandlers`. |
| Checkpoint | The last position a projector or event handler has finished. |
| Claim | A unique value reserved on the write side in the same transaction as the events (`user.email = a@b.c`). |

## How a request flows

```
controller ─SendCommand─▶ command bus ─▶ trace · log · metrics · wait · retry · authorize · validate ─▶ command handler
                                                                                                         │ load aggregate: snapshot + later events, upcast, apply
                                                                                                         │ aggregate method records new events
                                                                                                         ▼
                                                   one transaction: lock · check version · insert events · claims · snapshot · NOTIFY
                                                                                                         │
             worker ◀── LISTEN / poll ── events after each checkpoint ───────────────────────────────────┘
               ├─ projector:     read-model rows + checkpoint, one transaction
               └─ event handler: side effect + checkpoint, one transaction

controller ─SendQuery──▶ query bus ─▶ trace · log · metrics · authorize · validate ─▶ query handler ─▶ read-model tables
```

The `wait` step runs after the handler: it holds the response until every projector has passed
the position the command wrote, up to `CQRS_CONSISTENCY_TIMEOUT`.

## Commands and queries

Every use-case method becomes a message. Twelve commands and thirteen queries; `GetConversion`
and `GetPasskey` are new and internal (no new endpoint), because a write's response body comes
from a query (rule below).

| Domain | Commands | Queries |
| --- | --- | --- |
| task | `CreateTask`, `UpdateTask`, `TransitionTask`, `DeleteTask` | `GetTask`, `ListTasks`, `ListAllTasks` |
| user | `RegisterUser` | `LogIn`, `GetUser` |
| exchange | `ConvertCurrency` | `GetExchangeHistory`, `GetConversion` |
| passkey | `FinishPasskeyRegistration`, `RenamePasskey`, `DeletePasskey`, `FinishPasskeyLogin` | `BeginPasskeyRegistration`, `BeginPasskeyLogin`, `ListPasskeys`, `GetPasskey` |
| oidc | `FinishOIDCLogin` | `BeginOIDCLogin`, `GetOIDCStatus` |
| mail | `DeliverEmail` | — |

Rules:

- **A command returns identifiers, a position, or a token, never resource data.** A response body
  that shows a resource always comes from a query: REST `POST /v1/tasks` sends `CreateTask`, then
  `GetTask` with the returned ID.
- **The passkey and OIDC begin steps are queries.** They change nothing: the signed token already
  carries the ceremony state between begin and finish.
- **Mail is a command with no aggregate.** It holds no state; delivering is a side effect.
- Every behaviour of today's use cases is kept, error for error, except those listed under
  **Behaviour changes**.

## Events

All at schema version 1. Type names are the Go type names.

| Aggregate | Stream | Events |
| --- | --- | --- |
| `TaskAggregate` | `task-<uuid>` | `TaskCreated`, `TaskUpdated`, `TaskTransitioned`, `TaskDeleted`, `TaskImported` |
| `UserAggregate` | `user-<uuid>` | `UserRegistered`, `UserRegisteredViaOIDC`, `UserImported` |
| `PasskeyAggregate` | `passkey-<credential id, base64url>` | `PasskeyRegistered`, `PasskeyRenamed`, `PasskeyUsed`, `PasskeyDeleted`, `PasskeyImported` |
| `ConversionAggregate` | `conversion-<uuid>` | `CurrencyConverted`, `ConversionImported` |

- `*Imported` events exist only for the backfill and carry an aggregate's whole state.
- Each stored event also carries metadata: the caller's user ID, the trace ID (correlation), and
  the ID of the command or event that caused it (causation).
- Aggregates are named `<Name>Aggregate` because `entity.Task`, `entity.User`, `entity.Passkey` and
  `entity.Conversion` stay the read shapes: Huma names OpenAPI schemas after Go types and gqlgen
  binds `Task` to `entity.Task`, so renaming them would change the API.
- Rules move onto the aggregates (`TaskAggregate.Transition`); `TaskStatus.Valid` stays.

## Storage

All in the one database, declared in `internal/app/schema/*.hcl`.

| Table | Side | Holds |
| --- | --- | --- |
| `events` (new) | write | `position` (identity, primary key), `stream_id`, `stream_version`, `type`, `schema_version`, `payload jsonb`, `metadata jsonb`, `recorded_at`; unique `(stream_id, stream_version)` |
| `snapshots` (new) | write | latest snapshot per stream: `stream_id`, `stream_version`, `type`, `schema_version`, `payload`, `taken_at` |
| `claims` (new) | write | `scope`, `value`, `owner_id`; primary key `(scope, value)`. Scopes: `user.email`, `user.username`, `oidc.identity` |
| `webauthn_used_challenges` | write | unchanged: the login-replay guard, written in the command's transaction |
| `projection_checkpoints` (new) | read | `name`, `position`, `updated_at`; also the backfill marker |
| `tasks`, `users`, `user_identities`, `webauthn_credentials`, `exchange_history` | read | today's tables, now written only by projectors |

- **Read-model tables drop their foreign keys.** Each projector advances on its own, so the task
  projector can be ahead of the user projector; a foreign key between their tables would then fail.
- `exchange_history` gains a `conversion_id uuid` column for `GetConversion`.
- No table is removed, so Atlas's declarative apply never drops data.
- Streams whose ID starts with an underscore are scratch streams: stored and loadable, but never
  delivered to projectors or event handlers. The event store's own integration tests write only
  those, so they share the one database with a running worker without stalling it on event types
  it does not know.

### Appending

One transaction per command:

1. `pg_advisory_xact_lock` on the store's key, so positions commit in order and a projector that
   reads `position > checkpoint` never skips an event committed late.
2. Check the stream's version against the version the aggregate was loaded at. A mismatch is
   `entity.ErrConcurrencyConflict`.
3. Insert the events, then the claims (a taken claim is the domain's own error, such as
   `entity.ErrUserAlreadyExists`) and any guard rows.
4. Save a snapshot when the stream crosses a multiple of `EVENTSTORE_SNAPSHOT_EVERY`.
5. `NOTIFY events`; Postgres delivers it on commit.

The lock means one append at a time across the store. That is the price of in-order positions
without a second mechanism; documented in `docs/cqrs.md`.

### Loading

Latest snapshot (ignored when its schema version is not the aggregate's current one), then every
later event, each passed through the upcasters for its type until it reaches the current version,
then applied in order. A stream with no events is the domain's not-found error.

### Upcasters

A registry in the event store's codec maps `(type, from version)` to a function that rewrites the
payload to the next version. Every event starts at version 1, so none ship; a test registers a
fake version 0 → 1 upcaster and proves old events load.

## Projections and event handlers

- **Runner** (`internal/repo/persistent/projection`): one goroutine per subscriber in the worker.
  - Takes `pg_try_advisory_lock` on the subscriber's name, so with several worker replicas only one
    runs each subscriber.
  - Reads up to `PROJECTION_BATCH_SIZE` events after its checkpoint. It applies each event and
    moves the checkpoint in one transaction; the transaction travels in the context, so the
    subscriber's writes join it.
  - Wakes on `LISTEN events`, and polls every `PROJECTION_POLL_INTERVAL` as a fallback.
  - The checkpoint update is conditional on the old value, so a rebuild that reset it makes the
    runner reload instead of overwriting.
  - Events a subscriber does not handle still move its checkpoint.
- **Projectors**, one per domain: `task` (`tasks`), `user` (`users`, `user_identities`),
  `passkey` (`webauthn_credentials`), `exchange` (`exchange_history`).
- **Event handler:** `SendWelcomeEmail` on `UserRegistered` (not on `UserImported` or
  `UserRegisteredViaOIDC`, matching today). It enqueues on the `mail` queue inside the runner's
  transaction, so the email is enqueued exactly once.
- **Rebuild:** `make projection-rebuild name=task` empties that projector's tables and resets its
  checkpoint to 0 in one transaction; the running worker then replays from the first event.
- **Lag:** a `projection_lag_events{projector}` gauge on `/metrics`.

## Buses

`pkg/cqrs` holds the mechanism and the steps that need no domain knowledge. It imports nothing
from `internal/`.

```go
// Command is a request to change state; its name labels traces, logs and metrics.
type Command interface{ CommandName() string }

// Query is a request to read state.
type Query interface{ QueryName() string }

// Handler handles one message type.
type Handler[M, R any] interface {
	Handle(ctx context.Context, msg M) (R, error)
}

// Written is what a command handler returns: the aggregate it wrote and the position of its last event.
type Written struct {
	ID       string
	Position int64
}

// Commands and Queries are separate buses, so a query cannot be sent where a command belongs.
func NewCommands(steps ...Step) *Commands
func NewQueries(steps ...Step) *Queries

// HandleCommand registers h for C; a second handler for the same type panics at startup.
func HandleCommand[C Command, R any](b *Commands, h Handler[C, R])
func HandleQuery[Q Query, R any](b *Queries, h Handler[Q, R])

// SendCommand runs cmd through the pipeline to its handler. A type with no handler is ErrNoHandler.
func SendCommand[R any, C Command](ctx context.Context, b *Commands, cmd C) (R, error)
func SendQuery[R any, Q Query](ctx context.Context, b *Queries, q Q) (R, error)
```

There is no reflection-based discovery. Each domain lists its handlers in one function,
`internal/usecase/<domain>/register.go`, and `internal/app` calls each domain's `Register` once.

**Pipeline steps:**

| Step | Package | Commands | Queries | Does |
| --- | --- | --- | --- | --- |
| trace | `pkg/cqrs` | ✓ | ✓ | a span named `command CreateTask`; replaces every `internal/usecase/*/tracing.go` |
| log | `pkg/cqrs` | ✓ | ✓ | name, duration, outcome; the wrapped error on failure |
| metrics | `pkg/cqrs` | ✓ | ✓ | `cqrs_messages_total{kind,name,outcome}`, `cqrs_message_duration_seconds` |
| wait | `internal/usecase/behavior` | ✓ | | after the handler, polls the projector checkpoints until all pass `Written.Position`, up to `CQRS_CONSISTENCY_TIMEOUT`; past it, `entity.ErrReadModelBehind` |
| retry | `internal/usecase/behavior` | ✓ | | on `entity.ErrConcurrencyConflict`, re-runs the handler (which reloads the aggregate), three attempts in all |
| authorize | `internal/usecase/behavior` | ✓ | ✓ | a message with `RequiredRole() entity.Role` is checked against its `Caller`; `ListAllTasks` needs `RoleAdmin` (`entity.ErrAdminRequired`, as today) |
| validate | `internal/usecase/behavior` | ✓ | ✓ | calls `Validate() error` when the message has one; domain rules stay on the aggregate |

## Layout

```
pkg/cqrs/                                buses, Step, trace/log/metrics steps
pkg/postgres/                            + InTx(ctx, fn) and the context-carried transaction every repo uses

internal/entity/
  aggregate.go                           AggregateBase: ID, version, pending events
  event.go                               Event, RecordedEvent, Metadata, EventTypes() (the codec's registry)
  task.go                                Task (read shape, unchanged), TaskStatus, TaskAggregate
  task_events.go                         TaskCreated, TaskUpdated, TaskTransitioned, TaskDeleted, TaskImported
  … the same pair for user, passkey, exchange

internal/usecase/
  behavior/                              wait, retry, authorize, validate
  task/
    register.go                          Register(cmds, qs, deps): one line per handler
    command/create_task.go               CreateTask and its handler, one file per command
    command/create_task_test.go
    query/get_task.go                    GetTask and its handler, one file per query
    query/get_task_test.go
  user/event/send_welcome_email.go       event handlers, one file each
  (contracts.go and every tracing.go are removed)

internal/repo/
  contracts.go                           EventStore, ReadModel interfaces, Subscriber, QueueRepo, …
  persistent/eventstore/                 append, load, snapshots, claims, codec, upcasters, backfill
  persistent/projection/                 runner, checkpoints, rebuild
  persistent/task/                       TaskReadModel (sqlc), projector.go, tracing.go
  memstore/                              in-memory EventStore for command-handler tests
  repomock/                              gomock of repo/contracts.go (make mock)

internal/controller/                     every transport takes the two buses instead of use cases
  grpc/v1/router.go                      one NewRoutes(srv, cmds, qs, l) listing the domains, like REST
internal/app/                            builds the buses, calls each Register; the worker runs the subscribers
cmd/worker                               + `-rebuild <projector>`
```

Router signatures stop growing per domain: `restapi.NewRouter(cfg, cmds, qs, jwt, l, opts...)`,
`grpc.NewRouter(srv, cmds, qs, l)`, `graphql.NewHandler(cmds, qs, jwt, l)`. `WithPasskeys()` and
`WithOIDC()` still turn on the optional routes but carry no use case.

## Migrating existing data

At app start, before serving, inside an advisory lock so only one replica does it. Each domain
has its own importer and its own marker (`_backfill/task`, `_backfill/user`, …), so a domain's
rows get their streams in the same commit that moves that domain's writes onto the command bus.

For each importer whose marker is missing, in one transaction:

1. Read every row of the domain's tables:
   - task: `tasks`;
   - user: `users` and `user_identities`;
   - passkey: `webauthn_credentials`;
   - exchange: `exchange_history`.
2. Append one `*Imported` event per row, plus the claims for every email, username and identity.
3. Insert the marker and commit.

Projectors apply `*Imported` events as upserts: if the row already exists, nothing changes. So no
checkpoint is moved by hand, a projector that is behind is never skipped forward, and a rebuild
from zero recreates the rows from the same events. An empty database only gets the markers.

## Configuration

New env vars, each with a default in `config/config.go`, `.env.example` and
`docs/configuration.md`. None is an on/off switch.

| Variable | Default | Read by |
| --- | --- | --- |
| `CQRS_CONSISTENCY_TIMEOUT` | `2s` | app: the wait step |
| `EVENTSTORE_SNAPSHOT_EVERY` | `100` | app: appends |
| `PROJECTION_POLL_INTERVAL` | `1s` | worker |
| `PROJECTION_BATCH_SIZE` | `100` | worker |

## Behaviour changes

The API contracts do not change. These behaviours do:

1. **The worker is required.** Without it, every write commits but answers 503 (gRPC
   `Unavailable`, GraphQL `UNAVAILABLE`) after `CQRS_CONSISTENCY_TIMEOUT`. `make dev`, compose and
   the CI integration job already run it; `make run` alone is no longer enough
   (`docs/development.md`).
2. **Concurrent writes to one aggregate conflict instead of the last one winning.** After three
   attempts: 409, gRPC `Aborted`, GraphQL `CONFLICT`.
3. **`RegisterUser` no longer fails when the welcome email cannot be enqueued.** The email is
   enqueued exactly once by the event handler, instead of in a second step that could fail after
   the user was stored.
4. **Read-model tables have no foreign keys.**
5. **Appends run one at a time** across the store.

## Commands

```
make bin-deps                       # tool binaries
make sqlc                           # after the .sql or .hcl changes (needs Docker)
make mock                           # writes internal/repo/repomock
make format
make linter-golangci
make test
make compose-up-integration-test
make openapi && make web-api        # must produce no diff
make web-graphql && make web-check  # must produce no diff, must pass
make pre-commit                     # before every push
make projection-rebuild name=task   # new
```

## Code style

A command and its handler, one file:

```go
// CreateTask creates a task owned by UserID.
type CreateTask struct {
	UserID      string
	Title       string
	Description string
}

// CommandName -.
func (CreateTask) CommandName() string { return "CreateTask" }

type createTaskHandler struct {
	store repo.EventStore
	now   func() time.Time
}

func (h createTaskHandler) Handle(ctx context.Context, cmd CreateTask) (cqrs.Written, error) {
	task, err := entity.NewTaskAggregate(uuid.NewString(), cmd.UserID, cmd.Title, cmd.Description, h.now())
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("CreateTask - entity.NewTaskAggregate: %w", err)
	}

	position, err := h.store.Save(ctx, task)
	if err != nil {
		return cqrs.Written{}, fmt.Errorf("CreateTask - h.store.Save: %w", err)
	}

	return cqrs.Written{ID: task.ID(), Position: position}, nil
}
```

An aggregate records an event and applies it; replay uses the same `apply`:

```go
func (t *TaskAggregate) Transition(to TaskStatus, at time.Time) error {
	if !t.status.CanMoveTo(to) {
		return ErrInvalidTransition
	}

	t.record(TaskTransitioned{TaskID: t.ID(), From: t.status, To: to, At: at})

	return nil
}
```

A projector switches on the event type and writes through the transaction in the context:

```go
func (p Projector) Handle(ctx context.Context, e entity.RecordedEvent) error {
	q := queries.New(p.pg.Executor(ctx))

	switch ev := e.Event.(type) {
	case entity.TaskCreated:
		return q.InsertTask(ctx, insertTaskParams(ev))
	case entity.TaskDeleted:
		return q.DeleteTask(ctx, ev.TaskID)
	default:
		return nil
	}
}
```

A controller sends the command, then reads the resource back:

```go
written, err := cqrs.SendCommand[cqrs.Written](ctx, r.cmds, command.CreateTask{
	UserID: userID, Title: in.Body.Title, Description: in.Body.Description,
})
if err != nil {
	return nil, r.mapError(err, "createTask")
}

task, err := cqrs.SendQuery[entity.Task](ctx, r.qs, query.GetTask{UserID: userID, TaskID: written.ID})
```

Existing conventions hold: `New` constructors, `_`-prefixed package constants, the
`<Type> - <Method> - <call>: %w` error wrap, sentinels in `internal/entity/errors.go`, `t.Parallel()`
in every test.

## Testing strategy

| Level | Where | How |
| --- | --- | --- |
| Aggregates | `internal/entity/<name>_test.go` | given these events, when this method, then these events or this error |
| Command handlers | beside each handler | `memstore`: seed events, send the command, assert the appended events and claims |
| Query handlers | beside each handler | `repomock` read models |
| Buses and steps | `pkg/cqrs`, `internal/usecase/behavior` | each step on its own, then the full pipeline order |
| Codec, upcasters, snapshots | `persistent/eventstore` | unit, no database |
| Projectors | beside each projector | against a mocked sqlc `Querier` (`emit_interface: true`) |
| Controllers | the existing domain-folder tests | the real buses with fake handlers; today's assertions unchanged |
| End to end | `integration-test/` | every existing test unchanged and green, plus: read-after-write on every transport, a projection rebuilt and read back, a concurrent update conflicting, the welcome email exactly once, and the backfill (the test writes rows the old way, removes the domain's marker and runs the importer against the live database, which the suite already reaches through `PG_URL`); the event-store contract suite against Postgres |

## Boundaries

- **Always:**
  - keep the OpenAPI document, the protos and the GraphQL schema byte-identical;
  - one database;
  - drive everything through the Makefile;
  - tests beside the code;
  - one commit per domain;
  - update the docs listed below in the same PR.
- **Ask first:**
  - any API contract change;
  - a new Go dependency;
  - removing a table;
  - an env var beyond the four above;
  - any CI workflow change (none is expected: the integration job already starts the worker).
- **Never:**
  - a second database or schema;
  - markers in the code;
  - hand-editing generated files;
  - touching `tasks/`;
  - dropping any of the twelve parts to get to green;
  - Claude attribution in commits or the PR.

**Docs updated in this PR:**
- `docs/cqrs.md` (new);
- `docs/architecture.md`: Rules, Packages;
- `docs/request-flow.md` (it quotes the Convert path);
- `docs/worker.md`;
- `docs/configuration.md`;
- `docs/development.md`;
- CLAUDE.md: dependency rule, where code goes, the handler contract, traced decorators, mocks, both "Adding…" recipes;
- the Clean Architecture section of `README.md`.

## Success criteria

The twelve parts, each checkable:

1. Every one of the 12 commands is its own type and handler, one file each.
2. Task, user, passkey and conversion state is changed only by aggregates recording events.
3. `events` is append-only, and two writers on one stream conflict on `(stream_id, stream_version)`.
4. Every event has a schema version, and a test loads an older stored version through a chain of
   upcasters.
5. A stream past `EVENTSTORE_SNAPSHOT_EVERY` loads from its snapshot; a test proves the snapshot
   path and the full replay give the same aggregate.
6. Every one of the 13 queries is its own type and handler, reading only read-model tables.
7. Every read-model table is written only by its projector, and `make projection-rebuild` restores
   it identically (integration test).
8. The welcome email is sent by `SendWelcomeEmail` on `UserRegistered`, exactly once.
9. Every controller call goes through `SendCommand` or `SendQuery` and the pipeline. No
   `internal/usecase/*/tracing.go` or `usecase/contracts.go` remains.
10. All six domains are converted.
11. The tests in the table above exist and pass.
12. The docs above are updated.

**And the gates:**
- `make pre-commit` passes.
- `make compose-up-integration-test` passes.
- The CI checks are green.
- `make openapi`, `make web-api` and `make web-graphql` leave no diff.
- An existing local database keeps every row after the backfill, with reads identical before and
  after.

## Open questions

1. **When the wait times out, should the write answer 503 (as above) or answer from the
   aggregate's own state?** 503 keeps "bodies come from queries" pure, but a client that retries a
   timed-out `CreateTask` creates a second task. Answering from the aggregate never fails that way,
   but it breaks the rule for that one path.
2. **Should the commit be `refactor!` instead of `refactor`?** API contracts don't change, so plain
   `refactor` releases nothing. But the worker becomes required and four behaviours change.
   `refactor!` makes this the next major release (`docs/releasing.md`).
3. ~~Where should the plan go?~~ **Resolved:** in `docs/specs/cqrs-plan.md`, with the tasks in
   `docs/specs/cqrs-todo.md`. `tasks/` holds the unfinished release-system plan, with 20 and 17 open
   items, and stays untouched.
