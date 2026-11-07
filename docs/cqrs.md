# CQRS and event sourcing

Every domain is split into **commands**, which change state, and **queries**, which read it. The
state itself is **event-sourced**: the stored truth is the events each aggregate recorded, and the
tables the queries read are **projections** of those events, kept up to date by the worker.
[request-flow.md](request-flow.md) follows one command and one query through every layer.

## Vocabulary

| Term | Meaning here |
| --- | --- |
| Command | A request to change state (`CreateTask`). Handled by exactly one handler. Returns what identifies the write, never resource data. |
| Query | A request to read state (`GetTask`). Handled by exactly one handler, which reads only read-model tables. |
| Event | A fact that happened (`TaskCreated`). Past tense, immutable, stored forever. |
| Aggregate | The write-side object that takes a command, checks its rules and records events (`TaskAggregate`). |
| Stream | One aggregate's events, in order: `task-<id>`. Its length is the aggregate's version. |
| Position | An event's place in the whole store, increasing across all streams. |
| Projector | Turns events into read-model rows: the `task` projector writes `tasks`. |
| Event handler | Reacts to an event with a side effect: `SendWelcomeEmail` enqueues the welcome email. |
| Checkpoint | The last position a projector or event handler has finished. |
| Claim | A unique value reserved on the write side, in the same transaction as the events (`user.email`). |

## The pieces

```
controller ─SendCommand─▶ command bus ─▶ steps ─▶ command handler ─▶ aggregate ─▶ event store
                                                                                       │ NOTIFY
             worker: projection runner ◀─── events after each checkpoint ◀─────────────┘
               ├─ projectors       → read-model tables
               └─ event handlers   → side effects (the mail queue)
controller ─SendQuery──▶ query bus ─▶ steps ─▶ query handler ─▶ read-model tables
```

### Buses and steps (`pkg/cqrs`, `internal/usecase/behavior`)

- A message type embeds `cqrs.Command[R]` or `cqrs.Query[R]`, naming what it answers. `cqrs.SendCommand(ctx,
  cmds, cmd)` and `cqrs.SendQuery(ctx, qs, q)` find the one handler registered for the message's type,
  run it through the bus's steps and return an `R`: a sender never restates the result type, and a
  handler that answers another type does not compile. The type's Go name labels the message in
  traces, logs, metrics and the events' causation metadata.
- A message type with no handler is `cqrs.ErrNoHandler`.
- A second handler for one type panics at startup.
- The two buses are separate types, so a query can never be sent where a command belongs.
- There is no reflection-based discovery. Each domain registers its handlers in
  `internal/usecase/<domain>/register.go`, and each domain's module in `internal/app/<domain>.go` calls its
  `Register` once.

The steps, outermost first:

| Step | Commands | Queries | Does |
| --- | --- | --- | --- |
| trace | ✓ | ✓ | one span per message, `command CreateTask` |
| log | ✓ | ✓ | outcome and duration at debug |
| metrics | ✓ | ✓ | `cqrs_messages_total{kind,name,outcome}`, `cqrs_message_duration_seconds` |
| stamp | ✓ | ✓ | the actor (`ActorID()`) and the message name into the event metadata |
| wait | ✓ | | after the handler: until every projector has passed the write's position, or `CQRS_CONSISTENCY_TIMEOUT` (then `entity.ErrReadModelBehind`) |
| retry | ✓ | | runs the handler again on `entity.ErrConcurrencyConflict`, three attempts in all |
| authorize | ✓ | ✓ | an `AdminOnly()` message is refused with `entity.ErrAdminRequired` for a caller who is not an admin (`ListAllTasks`) |
| validate | ✓ | ✓ | calls `Validate()` when the message has one |

### Commands

One file per command, in `internal/usecase/<domain>/command/`. The file holds the command type and
its handler.

The handler returns `entity.Written`: the aggregate's ID and the position of its last event (zero
when it appended nothing). A sign-in embeds it beside the auth token (`SignedIn`); the wait step finds
it in either through the `entity.Write` interface. A command that writes no events, such as
`DeliverEmail`, answers `struct{}`, and nothing waits for it. A transport that answers with the resource
sends a query after the command: REST `POST /v1/tasks` sends `CreateTask`, then `GetTask` with the
returned ID.

The shared helpers in `internal/usecase/aggregate` do the store work:
- `aggregate.Save` saves what an aggregate recorded, new or loaded, and answers `Written` with the
  aggregate's own `ID()`;
- `aggregate.Change(ctx, store, id, func(t *entity.TaskAggregate) error {...})` loads one, runs the
  change and saves it. The aggregate type says the rest: `StreamOf(id)` names its stream and
  `NotFound()` is its error for an ID with none;
- `aggregate.Now` is the time in UTC, to the microsecond the read model keeps.

### Queries

One file per query, in `internal/usecase/<domain>/query/`. A query reads its domain's read model
interface (`repo.TaskReadModel`, …) and nothing else. It never loads an aggregate.

The begin steps of the passkey and SSO ceremonies are queries: they record nothing, since the token
they return carries the ceremony's state.

### Aggregates and events (`internal/entity`)

| Aggregate | Stream | Events |
| --- | --- | --- |
| `TaskAggregate` | `task-<uuid>` | `TaskCreated`, `TaskUpdated`, `TaskTransitioned`, `TaskDeleted`, `TaskImported` |
| `UserAggregate` | `user-<uuid>` | `UserRegistered`, `UserRegisteredViaOIDC`, `UserImported` |
| `PasskeyAggregate` | `passkey-<credential id>` | `PasskeyRegistered`, `PasskeyRenamed`, `PasskeyUsed`, `PasskeyDeleted`, `PasskeyImported` |
| `ConversionAggregate` | `conversion-<uuid>` | `CurrencyConverted`, `ConversionImported` |

- **Aggregates record and apply events.** An aggregate embeds `entity.AggregateBase`. Its behavior
  methods check the rules and call `entity.Record`, which applies the event and queues it for the
  store. `Apply` changes the state for one event, the same way whether the event was just recorded
  or is being replayed.
- **An aggregate keeps only what its rules need, in a private state.** `TaskAggregate` keeps the
  task's owner and status, `PasskeyAggregate` its owner and credential record, and `UserAggregate`
  and `ConversionAggregate`, which are never loaded, keep what their claims and stream need. The
  read shapes, `entity.Task`, `entity.User`, `entity.Passkey` and `entity.Conversion`, are what
  queries return and the API sends; renaming one of their fields changes no stored event or
  snapshot. They are named after the API schema, so the aggregates carry an `Aggregate` suffix.
- **A snapshot is the private state's JSON.** Only an aggregate that is loaded (task, passkey)
  snapshots. A field added to the state is missing from older snapshots and reads as its zero
  value; a change that cannot read the old shape raises `SnapshotVersion`, and older snapshots are
  then ignored and the stream replayed.
- **Every event type is registered.** Each one is listed in `entity.EventTypes()`, which the
  store's codec reads.

### The event store (`internal/repo/persistent/eventstore`)

`Save` appends an aggregate's pending events in one transaction, joining the transaction its
context carries:

1. **Take the store's advisory lock.** Appends run one at a time, so positions commit in the order
   they are assigned, and a reader that has passed a position never sees an earlier one commit
   later.
2. **Check the stream's version** against the version the aggregate was loaded at. A mismatch is
   `entity.ErrConcurrencyConflict`.
3. **Insert the claims and events.** A claim another owner holds is that claim's own `Taken` error,
   such as `entity.ErrUserAlreadyExists`.
4. **Save a snapshot** when the stream crosses a multiple of `EVENTSTORE_SNAPSHOT_EVERY`.
5. **`NOTIFY events`.**

`Load` reads the latest snapshot, when the aggregate's snapshot version matches, then the events
after it.

Two more features of the store:
- **Versioning.** Every event has its own `SchemaVersion`. When a shape changes, raise that event's
  version and add
  an `entity.Upcast` from the version before to `entity.Upcasts()`, beside the events. Stored events are never
  rewritten; old ones are upcast one version at a time as they load.
- **Write-side guards.** Claims (`claims` table) keep emails, usernames and SSO identities unique.
  `webauthn_used_challenges` keeps a passkey login from completing twice. A command writes both in
  its own transaction, so they are consistent at once. They are not projections.

### Projections and event handlers (`internal/repo/persistent/projection`)

The worker runs one loop per subscriber:

| Subscriber | Kind | Writes |
| --- | --- | --- |
| `task` | projector | `tasks` |
| `user` | projector | `users`, `user_identities` |
| `exchange` | projector | `exchange_history` |
| `passkey` | projector | `webauthn_credentials` |
| `send-welcome-email` | event handler | the `mail` queue |

- **Each loop reads in order.** It reads the events after its checkpoint, in position order, and
  hands the subscriber those of the types it `Handles`. The others move its checkpoint without
  reaching it, a batch of them in one transaction.
- **Every event reaches a read model.** The worker refuses to start when an event type in
  `entity.EventTypes()` has no projector, when two subscribers share a name, or when a name starts
  with `_` (`projection.Check`).
- **An event and its checkpoint commit together.** Each event is handled in one transaction with the
  checkpoint's move: a subscriber writes through `Executor(ctx)`, so its rows, or its queue message,
  commit with the checkpoint exactly once. A failure rolls both back, and the event is retried.
- **Several workers can run.** A transaction-scoped advisory lock per subscriber stops two workers
  handling the same subscriber at once. The checkpoint moves only from the position it was read at,
  so even a lost lock cannot apply an event twice.
- **The loop wakes on notification.** It wakes on `LISTEN events`, and polls every
  `PROJECTION_POLL_INTERVAL` as a fallback.
- **The worker can start before the app.** If the schema doesn't exist yet, it keeps retrying until
  the app has applied it.
- **`projection_lag_events{projector}`** on the app's `/metrics` is how far each subscriber is behind.

Read-model tables have no foreign keys: each projector advances on its own, so one can be ahead of
another.

### Rebuilding a projection

```sh
make projection-rebuild name=task
```

This empties the projector's tables and resets its checkpoint to 0, in one transaction, then
notifies the running worker, which replays every event into them. Until it catches up, reads from
those tables are incomplete. Only projectors can be rebuilt: replaying an event handler would
repeat its side effects, such as sending every welcome email again.

### Reads after writes

A command's response waits until every projector has passed its position, so a read sent right
after a write sees it. The web client relies on this: it reloads its lists after every mutation.

If the projectors are still behind after `CQRS_CONSISTENCY_TIMEOUT`, the write is saved, but the
answer is a 503 (gRPC `Unavailable`, GraphQL `UNAVAILABLE`): read again shortly, don't write again.
A write can only answer like this when the worker is down or far behind, so the app needs the
worker running.

### The backfill

At start, the app runs each domain's importer once, under the store's lock, in one transaction with
its marker (`_backfill/<domain>` in `projection_checkpoints`). An importer turns the rows the domain
wrote before it stored events into `*Imported` events, one per row, and claims what they hold.

Projectors apply `*Imported` events as upserts, so the rows stay as they were and no checkpoint is
ever moved by hand.

### Scratch streams

A stream whose ID starts with an underscore is stored and loadable but never delivered to a
subscriber. The event store's own integration tests write only those, so they can share the one
database with a running worker.

## Adding to it

- **A command.** Add `internal/usecase/<domain>/command/<name>.go`, holding the type, which embeds
  `cqrs.Command[entity.Written]` (and has `ActorID()` when it acts for a user), and a handler returning
  `entity.Written`.
  Put its test beside it, against `internal/repo/memstore`. Add one line to the domain's
  `register.go`. Then send it from each transport.
- **A query.** Add `internal/usecase/<domain>/query/<name>.go` and a test against
  `internal/repo/repomock`. Add one line to `register.go`.
- **An event.** Add the type to `internal/entity/<domain>_events.go` with `EventType()` and
  `SchemaVersion()`, add it to the domain's list in `EventTypes()`, and handle it in the aggregate's
  `Apply` and in the projector.
- **A changed event.** Raise its `SchemaVersion` and add the upcast from the previous version.
- **A projector.** Add `internal/repo/persistent/<domain>/projector.go`. It needs `Name`, `Tables`,
  `Handles` (its domain's event list, such as `entity.TaskEvents()`) and a `Project` function, with a
  `default` case answering `entity.ErrUnhandledEvent`, tested against a fake of sqlc's `Querier`.
  Return it from the `projector` of the domain's module (`internal/app/<domain>.go`).
- **An event handler.** Add `internal/usecase/<domain>/event/<name>.go`, with a `Name`, the events it
  `Handles` and a `Handle` that does its work through repositories that join the context's
  transaction. Add it to `eventHandlers`.

## Configuration

See [configuration.md](configuration.md#cqrs): `CQRS_CONSISTENCY_TIMEOUT`,
`EVENTSTORE_SNAPSHOT_EVERY`, `PROJECTION_POLL_INTERVAL`, `PROJECTION_BATCH_SIZE`.

## What it changed

- **The worker is required.** Without it, writes answer 503 after the consistency timeout.
- **Concurrent writes to one record conflict** instead of the last one winning. A write that still
  loses after the retries answers 409 (gRPC `Aborted`, GraphQL `CONFLICT`).
- **Registration does not fail when the welcome email cannot be enqueued.** The event handler
  enqueues it exactly once.
- **The read-model tables have no foreign keys.**
- **Appends run one at a time** across the store.
