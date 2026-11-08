# Spec: the queue on NATS JetStream

Module id: `nats-queue` · first of four (see [README.md](README.md#observability-and-messaging)) ·
Depends on `goose` · Status: **approved 2026-11-08 except the outbox**, which has its own session
before the plan; work starts after the Mongo modules and `goose`

Paths below are checked against `feat/valkey` at `79e68ed` on 2026-11-08.

## Objective

The worker's queue leaves Postgres for a broker. Messages are delivered by **NATS JetStream**
through **Watermill**, and still leave the app atomically with the write that caused them, through
an outbox in Postgres. This fixes what the homegrown queue lacks: a retry limit with backoff, a
give-up state that keeps the message, errors that never retry, and a panic in one handler taking the
worker down. When it is done:

- `queue_messages`, `internal/repo/persistent/queue` and `pkg/worker` are gone;
- the app writes a message only to the outbox, in the transaction it already has, and never
  connects to NATS;
- the worker runs Watermill's forwarder (outbox → NATS) and a router with one handler per topic,
  like an HTTP route; mail is its first topic, unchanged for the user;
- a failing message is retried with exponential backoff up to a limit, then moved to the topic's
  poison topic and logged at `Error`; a message that can never succeed goes there at once;
- a panic in a handler is recovered and retried; the other topics keep running;
- every way of running the worker has a NATS: `make dev`, the compose stack, the integration stack
  and CI.

**Who it is for:** a developer starting from the template who needs background work that survives
failures, and sees the outbox pattern done properly.

## Tech stack

| Piece | Version | Why |
| --- | --- | --- |
| Broker | `nats` image with JetStream on (`-js`), exact tag pinned in the plan | ~20 MB, Go, retries with backoff built in, streams keep history for later integration events |
| Router and middleware | `github.com/ThreeDotsLabs/watermill`, pinned in the plan | router per topic; `Recoverer`, `Retry`, `PoisonQueue` middleware; the outbox `forwarder` |
| NATS adapter | `github.com/ThreeDotsLabs/watermill-nats` (JetStream) | publisher and subscriber |
| Outbox | **deferred to its own session** (2026-11-08): our own table + sqlc insert + relay, or `watermill-sql/v4` | Watermill's router does not need either |

Watermill's SQL tables are created by it only when `InitializeSchema` is true. It stays false: the
tables come from a goose migration (see [goose.md](goose.md)).

## Configuration

| Env var | Default | Read by | |
| --- | --- | --- | --- |
| `NATS_URL` | required by the worker | the worker | `nats://localhost:${NATS_PORT}` in `.env.example`, `nats://nats:4222` in compose |
| `NATS_PORT` | `4222` | compose, `.env.example` | the host port the compose `nats` service publishes |
| `WORKER_*` | today's | the worker | kept where they still mean something (retry, backoff, shutdown); the plan lists which go |

`NATS_URL` has no `required` tag: the app parses the same config and never uses it. `RunWorker`
returns `ErrNoNATSURL` without it, as `app.Run` does for `VALKEY_URL`.

## Design (decided)

1. **NATS is the worker's alone.** The app writes the outbox; the worker relays and consumes.
   NATS down delays mail; it never fails a request.
2. **The outbox is the only way out.** A port (today `notify.Queue`'s `Send`) publishes through
   Watermill's SQL publisher on the transaction in `ctx`, so a message from an event handler
   commits with its checkpoint, exactly as now.
3. **One router, one handler per topic** in `internal/controller/worker/v1/`, still the queue
   transport: decode, send a command, return the error.
4. **Retry and give up per topic.** Backoff, attempts and the poison topic are set where the topic is
   registered, from values in `config/`.
5. **`Permanent` errors** skip the retries: the handler that today returns `nil` on a body it cannot
   decode returns one instead, so the message is kept, not dropped.

## Project structure

| Where | Change |
| --- | --- |
| `pkg/natsjs/` | new: connect, ensure the streams, return the Watermill publisher/subscriber and a close |
| `pkg/worker/` | removed |
| `internal/app/migrations/` | a migration: drop `queue_messages`, create Watermill's message and offsets tables |
| `internal/repo/persistent/queries/queue.sql`, `internal/repo/persistent/queue/` | removed |
| `internal/repo/outbox/` (name in the plan) | the publisher on the transaction in `ctx`, behind the port `notify` uses |
| `internal/controller/worker/v1/mail.go` | a Watermill handler; `Permanent` on an undecodable body |
| `internal/app/worker.go`, `internal/app/modules.go` | the router, forwarder and middleware; `queues` register topics |
| `config/config.go`, `.env.example`, `docker-compose*.yml`, `Makefile`, `scripts/dev.sh`, `.github/workflows/ci.yml` | NATS |
| `integration-test/` | mail still reaches Mailpit; a message that always fails ends in the poison topic |
| `docs/worker.md`, `docs/configuration.md`, `docs/development.md`, `docs/architecture.md`, `docs/cqrs.md`, `README.md`, `CLAUDE.md` | the queue section rewritten |

## Testing strategy

- **Unit:** the mail handler with a fake command bus (decode error is `Permanent`, a send error
  retries); the middleware chain with Watermill's `gochannel` pub/sub, without Docker: retry count,
  backoff, poison topic, recover.
- **Integration:** a welcome email still arrives; NATS stopped then started, the email still
  arrives; a poison message is kept.
- **By hand:** the forwarder and handler spans in Jaeger.

## Boundaries

- **Always:** the repo's checks; pinned versions; the outbox written in the caller's transaction.
- **Ask first:** NATS in the app; a second broker; integration events to other services; dropping
  the outbox for direct publishes.
- **Never:** publishing to NATS from a command or event handler directly; `InitializeSchema: true`;
  a toggle between the old queue and the new one.

No upgrade path: messages left in `queue_messages` are not moved; the database is reset, as
[goose.md](goose.md) decides.

## Success criteria

1. The app starts and serves with no `NATS_URL`; the worker refuses to start without one.
2. A sign-up's welcome email reaches Mailpit through outbox, forwarder, NATS and the mail handler.
3. A handler that always fails is tried `MaxRetries+1` times with growing gaps, then its message is
   in the poison topic and an `Error` is logged.
4. A panic in a handler is logged and retried; mail keeps flowing.
5. `queue_messages`, `pkg/worker` and the queue repo are gone; checks and CI are green.

## TODO

- **TODO (owner: Mohammed):** how to deal with poison messages: how they are inspected, replayed or
  discarded (a `make` target, NUI, or something else). Out of scope for now: messages land in the
  topic's poison stream and are logged at `Error`, nothing more. When the code lands, the poison
  middleware's registration carries a `// TODO:` pointing here.

## Release

**Breaking** (decided 2026-11-08): the worker never starts without `NATS_URL`. The commit that makes
it so is `feat(worker)!:` with `BREAKING CHANGE: the worker needs NATS_URL and a NATS JetStream
server; reset the database (goose)`.

## Open questions

1. **The outbox** (deferred to a later session, 2026-11-08): our own one-table outbox (sqlc insert
   through `pg.Executor(ctx)`, a relay in the worker) or `watermill-sql`. Messages from domain
   events need none: the event handler publishes, the event store is their outbox. Only
   command-side messages (`user_logged_in`, `task_attributes_updated`) need it.
