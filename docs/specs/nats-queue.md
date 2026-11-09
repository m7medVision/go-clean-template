# Spec: the queue on NATS JetStream

Module id: `nats-queue` · first of four (see [README.md](README.md#observability-and-messaging)) ·
Depends on `goose` · Status: **done: merged 2026-11-09 (#56), released in v0.6.0** (approved 2026-11-08 except the outbox, which turned out not to be needed
here and keeps its own session; it was built before the Mongo modules, on top of `goose`)

Paths below are checked against `feat/valkey` at `79e68ed` on 2026-11-08.

## Objective

The worker's queue leaves Postgres for a broker. Messages are delivered by **NATS JetStream**
through **Watermill**. This fixes what the homegrown queue lacks: a retry limit with backoff, a
give-up state that keeps the message, errors that never retry, and a panic in one handler taking the
worker down. When it is done:

- `queue_messages`, `internal/repo/persistent/queue` and `pkg/worker` are gone;
- the worker runs a Watermill router with one handler per topic, like an HTTP route; mail is its
  first topic, unchanged for the user; the app never connects to NATS;
- a failing message is retried with exponential backoff up to a limit, then moved to the `poison`
  topic and logged at `Error`; a message that can never succeed goes there at once;
- a panic in a handler is recovered and retried; the other topics keep running;
- every way of running the worker has a NATS: `make dev`, the compose stack, the integration stack
  and CI.

**Who it is for:** a developer starting from the template who needs background work that survives
failures.

## No outbox in this module (found 2026-11-09)

The only producer today is the `send-welcome-email` event handler, which runs in the worker, and the
worker has the broker. For a message that follows a stored event, the event store already is the
outbox: the projection runner delivers the event at least once, the handler publishes, then its
checkpoint moves. A crash in between publishes again, and the message ID (`welcome-<user id>`)
makes JetStream drop the repeat. So nothing in this module needs an outbox table. The outbox
(own table or `watermill-sql`) is still deferred: it is needed only for messages a **command in the
app** sends with no stored event (PostHog's `user_logged_in`, `task_attributes_updated`).

## Tech stack

| Piece | Version | Why |
| --- | --- | --- |
| Broker | `nats:2.15.2-alpine`, JetStream on (`-js`) | ~20 MB, Go, streams keep history for later integration events |
| Router and middleware | `github.com/ThreeDotsLabs/watermill` v1.5.3 | router per topic; `Recoverer`, `Retry`, `PoisonQueue` middleware |
| NATS adapter | `github.com/ThreeDotsLabs/watermill-nats/v2` v2.2.0, its stable `pkg/nats` with JetStream on (its `pkg/jetstream` is marked experimental) | publisher and subscriber, durable queue groups, message-ID dedup |

## Configuration

| Env var | Default | Read by | |
| --- | --- | --- | --- |
| `NATS_URL` | required by the worker | the worker | `nats://localhost:${NATS_PORT}` in `.env.example`, `nats://nats:4222` in compose |
| `NATS_PORT` | `4222` | compose, `.env.example` | the host port the compose `nats` service publishes |
| `WORKER_MAX_RETRIES` | `5` | the worker | retries before the poison topic |
| `WORKER_RETRY_INITIAL`, `WORKER_RETRY_MAX` | `1s`, `30s` | the worker | the backoff, doubling |
| `WORKER_ACK_WAIT` | `5m` | the worker | JetStream redelivers after it; longer than a handler and its retries |
| `WORKER_SHUTDOWN_TIMEOUT` | `10s` | the worker | grace for in-flight handlers |

`WORKER_POLL_INTERVAL`, `WORKER_VISIBILITY_TIMEOUT` and `WORKER_BATCH_SIZE` are gone. `NATS_URL` has
no `required` tag: the app parses the same config and never uses it. `RunWorker` returns
`ErrNoNATSURL` without it, as `app.Run` does for `VALKEY_URL`.

## Design (decided)

1. **NATS is the worker's alone.** NATS down delays mail; it never fails a request.
2. **Producers publish through a port with a stable ID** (`notify.Publisher`, which
   `*broker.Broker` satisfies), so a repeat after a crash is deduplicated.
3. **One router, one handler per topic** in `internal/controller/worker/v1/`, still the queue
   transport: decode, send a command, return the error. Each topic is a durable consumer in queue
   group `worker-<topic>`, so worker replicas share it.
4. **Retry, then poison.** Middleware order: poison, give-up log, retry with backoff, recoverer.
5. **`broker.Permanent` errors** skip the retries: the handler that returned `nil` on a body it
   cannot decode returns one instead, so the message is kept, not dropped.

## Project structure

| Where | Change |
| --- | --- |
| `pkg/broker/` | new: Watermill on NATS JetStream (`New`, `Publish`, `Handle`, `Run`, `Close`, `Permanent`), with a test of the middleware chain on Watermill's in-memory pub/sub |
| `pkg/worker/` | removed |
| `internal/app/migrations/<timestamp>_drop_queue_messages.sql` | drops `queue_messages` (its Down recreates it) |
| `internal/repo/persistent/queries/queue.sql`, `internal/repo/persistent/queue/` | removed |
| `internal/repo/notify/` | `Publisher` port in place of `Queue`; the welcome's ID is `welcome-<user id>` |
| `internal/controller/worker/v1/mail.go` | a broker handler on a `Router`; `broker.Permanent` on an undecodable body |
| `internal/app/worker.go`, `modules.go`, `mail.go`, `user.go` | the broker in `deps`; `queues` register on a router |
| `config/`, `.env.example`, `docker-compose*.yml`, `Makefile`, `.github/workflows/ci.yml` | NATS and the new `WORKER_*` settings |
| `integration-test/queue_test.go`, `mail_test.go` | mail through NATS reaches Mailpit; an undecodable message lands on `poison` |
| `docs/worker.md`, `docs/configuration.md`, `docs/development.md`, `docs/architecture.md`, `docs/cqrs.md`, `README.md`, `CLAUDE.md` | the queue section rewritten |

## Testing strategy

- **Unit:** the mail handler with a fake command bus (decode error is `Permanent`, a send error
  retries); the middleware chain with Watermill's `gochannel` pub/sub, without Docker: retry count,
  poison topic, permanent, recover. Checked once by hand against a real NATS: two replicas, one
  delivery; a repeated ID delivered once.
- **Integration:** a welcome email still arrives, once; a message published to `mail` reaches
  Mailpit; an undecodable one lands on `poison` with its reason.
- **By hand:** the forwarder and handler spans in Jaeger.

## Boundaries

- **Always:** the repo's checks; pinned versions; the outbox written in the caller's transaction.
- **Ask first:** NATS in the app; a second broker; integration events to other services; dropping
  the outbox for direct publishes.
- **Never:** a vendor SDK outside `pkg/broker`; NATS in the app; a toggle between the old queue and
  the new one; `return nil` to drop a message.

No upgrade path: messages left in `queue_messages` are not moved; the database is reset, as
[goose.md](goose.md) decides.

## Success criteria

1. The app starts and serves with no `NATS_URL`; the worker refuses to start without one.
2. A sign-up's welcome email reaches Mailpit through the event handler, NATS and the mail handler.
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

1. **The outbox** (deferred to a later session, 2026-11-08; not needed by this module): our own
   one-table outbox or `watermill-sql`, for messages a command in the app sends with no stored
   event (`user_logged_in`, `task_attributes_updated`).
