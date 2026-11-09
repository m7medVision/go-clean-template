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
| Event handler | Reacts to an event with a side effect: `SendWelcomeEmail` publishes the welcome email. |
| Checkpoint | The last position a projector or event handler has finished. |
| Claim | A unique value reserved on the write side, in the same transaction as the events (`user.email`). |

## The pieces

```
controller ─SendCommand─▶ command bus ─▶ steps ─▶ command handler ─▶ aggregate ─▶ event store
                                                                                       │ NOTIFY
             worker: projection runner ◀─── events after each checkpoint ◀─────────────┘
               ├─ projectors       → read-model tables
               └─ event handlers   → side effects (the mail topic on NATS)
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

`newBuses` (`internal/app/buses.go`) builds the buses with their steps. Each binary calls it once:
the app with the limit step, the worker without it. The steps, outermost first:

| Step | Commands | Queries | Does |
| --- | --- | --- | --- |
| trace | ✓ | ✓ | one span per message, `command CreateTask` |
| log | ✓ | ✓ | outcome and duration at debug |
| metrics | ✓ | ✓ | `cqrs_messages_total{kind,name,outcome}`, `cqrs_message_duration_seconds` |
| stamp | ✓ | ✓ | the actor (`ActorID()`) and the message name into the event metadata |
| limit | ✓ | ✓ | the app's buses only, for a `behavior.Limited` message (`RegisterUser`, `LogIn`): the [sign-in limit](#the-sign-in-limit) |
| wait | ✓ | | after the handler: until the projectors that handle the events it wrote (`entity.Written.Events`) have passed its position, or `CQRS_CONSISTENCY_TIMEOUT` (then `entity.ErrReadModelBehind`) |
| retry | ✓ | | runs the handler again on `entity.ErrConcurrencyConflict`, three attempts in all |
| authorize | ✓ | ✓ | an `AdminOnly()` message is refused with `entity.ErrAdminRequired` for a caller who is not an admin (`ListAllTasks`) |
| validate | ✓ | ✓ | calls `Validate()` when the message has one |

### The sign-in limit

`behavior.Limit` limits the messages that implement `behavior.Limited` (`LimitScope`,
`LimitAccount`): `user/command.RegisterUser` (scope `sign-up`, no account) and `user/query.LogIn`
(scope `sign-in`, its email). Passkey and SSO sign-in are not limited. For each of those messages,
the step:

1. counts the attempt against the client's budget, if the context carries a client
   (`entity.ClientFromContext`, set by the transports' client middleware);
2. checks the account's budget without counting, when there is an account;
3. runs the rest of the pipeline and the handler;
4. after a wrong password (`entity.ErrInvalidCredentials`), counts one failure against the account;
   after a success, clears the account's count.

A spent budget refuses the attempt in step 1 or 2 with `entity.ErrTooManyAttempts`
(`entity.KindRateLimited`), carried in an `*entity.RetryAfterError` with the wait. Each transport
turns it into its status ([api.md](api.md#user-authentication)) and, as for the caller's other
mistakes, logs nothing. The step runs before authorize and validate, so a malformed attempt spends
the client's budget too. Checking and counting the account are separate calls, so concurrent wrong
passwords can pass a few over the budget before it locks. The budgets are the `AUTH_LIMIT_*`
settings ([configuration.md](configuration.md#sign-in-limit)).

The counters are behind `repo.Limiter` (`internal/repo/contracts.go`), implemented by
`internal/repo/limiter` on two `valkeylimiter` clients, one per budget, built on the app's Valkey
client and never closed on their own. Each is a fixed window, starting with the first attempt.
The keys are `limit:client:{<scope>:<addr>}` (an IPv6 client by its /64) and
`limit:account:{<scope>:<hash>}`, the hash being the hex SHA-256 of the trimmed, lower-cased email,
so Valkey holds no email address.

The limit fails open. Each limiter call gets 500 ms at most and does not inherit the caller's
cancellation. If one fails, the step logs the warning `sign-in limit unavailable` with the scope,
makes no more limiter calls for that attempt, and lets it through: a Valkey outage never stops
every sign-in.

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
- `aggregate.Load[entity.PasskeyAggregate](ctx, store, id)` loads one by ID. The aggregate type says
  the rest: `StreamOf(id)` names its stream, or answers the not-found error for an ID that names
  none (a passkey ID that is not base64url), and `NotFound()` is its error for an ID with no stream.
  Passkey registration takes that error to mean a new credential;
- `aggregate.Change(ctx, store, id, func(t *entity.TaskAggregate) error {...})` loads one as `Load`
  does, runs the change and saves it;
- `aggregate.Now` is the time in UTC.

### Queries

One file per query, in `internal/usecase/<domain>/query/`. A query reads its domain's read model
interface (`repo.TaskReadModel`, …) and nothing else. It never loads an aggregate. A read model
returns a record by its ID, whoever owns it, and the query decides whether the caller may read it
with the record's `ReadableBy` (`GetTask`, `GetPasskey`, `GetConversion`).

The task queries also read `repo.TaskAttributesStore` (MongoDB): `GetTask` one document, the lists
one `GetMany` per page, and they answer `mongotask.TaskWithAttributes`.

### The one write outside an aggregate: task attributes

`SetTaskAttributes` (`internal/usecase/task/command`) checks the task through the read model
(`ReadableBy`) and writes the attributes straight to MongoDB through `repo.TaskAttributesStore`. It
records no event and answers an `entity.Written` with none, so the wait step does not wait. That is
the cost of the exception: attributes have no history, no projector rebuilds them, and MongoDB must
be backed up on its own. `DeleteTask` deletes the task through the aggregate, then its document; a
MongoDB failure there is logged and the command still succeeds, leaving an unreachable document.
Use this shape only for data whose history nobody needs.

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
  rewritten; old ones are upcast one version at a time as they load. `internal/repo/eventcodec/testdata/events/`
  holds one frozen sample per event type and version, and the codec tests fail when an event's stored shape
  no longer matches its sample or an old sample no longer decodes.
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
| `send-welcome-email` | event handler | the `mail` topic on NATS |
| `capture-user`, `capture-task`, `capture-exchange` | event handlers, only with an analytics vendor | one `analytics-<sink>` topic per sink ([analytics.md](analytics.md)) |

- **Each loop reads in order.** It reads the events after its checkpoint, in position order, and
  hands the subscriber those of the types it `Handles`. The others move its checkpoint without
  reaching it, a batch of them in one transaction.
- **Every event reaches a read model.** The worker refuses to start when an event type in
  `entity.EventTypes()` has no projector, or when two subscribers share a name (`projection.Check`).
- **An event and its checkpoint commit together.** Each event is handled in one transaction with the
  checkpoint's move: a subscriber writes through `Executor(ctx)`, so its rows commit with the
  checkpoint exactly once. A side effect outside Postgres (a NATS publish) is at-least-once instead:
  it carries an ID the broker deduplicates. A failure rolls both back, and the event is retried.
- **Several workers can run.** A transaction-scoped advisory lock per subscriber stops two workers
  handling the same subscriber at once. The checkpoint moves only from the position it was read at,
  so even a lost lock cannot apply an event twice.
- **The loop wakes on notification.** It wakes on `LISTEN events`, and polls every
  `PROJECTION_POLL_INTERVAL` as a fallback.
- **A new event handler can skip the history.** A subscriber that implements `repo.StartsAtHead`
  gets its checkpoint at the store's last event the first time it runs, not at 0: the analytics
  handlers do, so configuring a vendor sends no past events. An existing checkpoint stays.
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
those tables are incomplete. It refuses while the backfill has not imported every domain: the
importers read those tables, so emptying one would lose the rows they have still to import. Only projectors can be rebuilt: replaying an event handler would
repeat its side effects, such as sending every welcome email again.

### Reads after writes

A command's response waits until the projectors that handle the events it wrote have passed its
position, so a read sent right after a write sees it. A projector that handles none of them is not
waited for, so one that is stuck holds up only the writes it projects. The web client relies on this: it reloads its lists after every mutation.

If the projectors are still behind after `CQRS_CONSISTENCY_TIMEOUT`, the write is saved, but the
answer is a 503 (gRPC `Unavailable`, GraphQL `UNAVAILABLE`): read again shortly, don't write again.
A write can only answer like this when the worker is down or far behind, so the app needs the
worker running.

### The backfill

At start, the app runs each domain's importer once, under the store's lock, in one transaction with
its marker (the domain's row in `backfill_markers`). An importer reads the rows the domain wrote
before it stored events and returns one new aggregate per row, holding its `*Imported` event, with
the claims the row holds. `eventstore.Backfill` saves them in one loop for every domain, and skips a
stream that exists already: its row came from its events. It checks before it saves, so a stream
that exists never fails a statement inside the import's transaction. A database upgraded from
v0.3, which kept its markers in `projection_checkpoints` as `_backfill/<domain>`, has them moved to
`backfill_markers` at the next start instead of importing again.

Projectors insert the rows of `*Imported` events with `ON CONFLICT DO NOTHING`, so a row that is
there already stays as it was and no checkpoint is ever moved by hand.

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
- **A changed event.** Raise its `SchemaVersion`, add the upcast from the previous version, and keep the old
  sample in `internal/repo/eventcodec/testdata/events/`. A new event, or a new version, gets its sample from
  `UPDATE_GOLDEN=1 go test -run TestWriteEventSamples ./internal/repo/eventcodec/`; review the new file.
- **A projector.** Add `internal/repo/persistent/<domain>/projector.go`: a `NewProjector(pg)` that
  returns `projection.NewProjector(name, tables, handles, pg, project)`, with its name, the tables a
  rebuild empties, the events it `Handles` (its domain's event list, such as `entity.TaskEvents()`)
  and an unexported `project` function: one case per event, writing through the `queries.Querier` it
  is handed, and a `default` case answering `entity.ErrUnhandledEvent`. Test `project` against a fake
  of sqlc's `Querier`, through the package's `export_test.go`. Return the projector from the
  `projector` of the domain's module (`internal/app/<domain>.go`).
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
- **Registration does not fail when the welcome email cannot be published.** The event handler
  publishes it once per user: a repeat carries the same message ID, which NATS drops.
- **The read-model tables have no foreign keys.**
- **Appends run one at a time** across the store.
