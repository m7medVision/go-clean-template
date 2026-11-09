# Queue and worker

The worker, `cmd/worker`, is a separate binary with two jobs. It runs the **projections**: the
projectors that keep the read models up to date from the event store, and the event handlers that
react to events. And it consumes a **queue**: topics on [NATS JetStream](https://docs.nats.io/nats-concepts/jetstream),
routed by [Watermill](https://watermill.io/). Only the worker connects to NATS; the app never does.
The welcome email after a registration uses both: an event handler publishes it, a topic handler
delivers it.

The app needs the worker: a write waits for the projectors before it answers
([cqrs.md](cqrs.md#reads-after-writes)).

## Projections

The worker starts one loop per subscriber, every module's projector and event handlers
(`internal/app/<domain>.go`), next to
the queue consumer, and stops them on the same SIGTERM; it closes the pool once both have returned. How a loop reads, commits and wakes is in
[cqrs.md](cqrs.md#projections-and-event-handlers-internalrepopersistentprojection); its settings
(`PROJECTION_POLL_INTERVAL`, `PROJECTION_BATCH_SIZE`) are in
[configuration.md](configuration.md#cqrs).

To rebuild one projector's read model from the events, with the worker running:

```sh
make projection-rebuild name=task
```

That runs `cmd/worker -rebuild task`, which empties the tables, resets the checkpoint, notifies the
running worker and exits; the worker replays every event into them. Only projectors can be rebuilt.

## The queue

[`pkg/broker`](../pkg/broker) wraps Watermill on NATS JetStream: `New` connects a publisher and a
subscriber to `NATS_URL`, `Publish(ctx, topic, id, body)` sends a message, `Handle(topic, handler)`
registers a route and `Run` serves them until the context ends. A topic is a JetStream stream of
the same name, created when it is first used.

- **One route per topic, like an HTTP route.** Each topic has a durable consumer in the queue group
  `worker-<topic>`, so several workers share a topic's messages and each message reaches one of
  them; a worker that restarts carries on from the first message not yet acknowledged.
- **At-least-once.** A message is acknowledged after its handler returns `nil`. Handlers must be
  idempotent. A publish carries an ID, and JetStream drops a second publish with the same ID inside
  its duplicate window, so a producer that runs again after a crash does not send twice.
- **Retry with backoff.** A handler that returns an error, or panics, is retried in the worker
  `WORKER_MAX_RETRIES` times, waiting from `WORKER_RETRY_INITIAL` and doubling up to
  `WORKER_RETRY_MAX`. `WORKER_ACK_WAIT` must be longer than a message's handling and all its
  retries, or JetStream redelivers it to another worker meanwhile.
- **Poison.** A message still failing after its retries, or one whose handler returns
  `broker.Permanent(err)` (a body that can never decode), is moved to the `poison` topic with the
  reason and its topic in its metadata, and logged at `Error`. It is kept, not dropped. How poison
  messages are inspected and replayed is not decided yet (the TODO in `pkg/broker`).
- **Shutdown.** On SIGTERM the router stops taking messages and waits up to
  `WORKER_SHUTDOWN_TIMEOUT` for the handlers in flight.
- **Startup.** `RunWorker` refuses to start without `NATS_URL` (`app.ErrNoNATSURL`). The app applies
  the schema, not the worker. It serves no HTTP, issues no token and reads no Valkey, so it starts
  without `JWT_SECRET` and `VALKEY_URL`, which the app requires.

The settings are in [configuration.md](configuration.md#worker).

## Handlers

Handlers live in [`internal/controller/worker/v1`](../internal/controller/worker/v1), each with a
function that registers it on a `v1.Router` (`r.Handle(topic, handler)`, which `*broker.Broker`
implements), called by its module's `queues` in `internal/app` before the worker runs. A message is
an inbound request like an HTTP call, so a handler has the same shape: decode the body, send a
command on the command bus with the handler's `ctx`, and return its error. The topics today are
`mail` (below), which `mail.go` delivers email from, and one `analytics-<sink>` per configured
analytics vendor, which `analytics.go` delivers to that vendor ([analytics.md](analytics.md)).

### Adding a topic

1. Handler: `internal/controller/worker/v1/<name>.go`, with the function that registers it, called
   from the `queues` of a module in `internal/app`, which also registers the command handlers its
   messages send to on the worker's buses. Return `broker.Permanent(err)` for a message that can
   never succeed.
2. Producer: give the event handler a port whose implementation calls
   `Publish(ctx, "<name>", id, body)` (through a small interface of its own, as `notify.Publisher`
   does), with an ID that is the same each time the handler runs for the same event. Producers run
   in the worker, which has the broker; a command in the app cannot publish, because the app has
   no NATS (an outbox for that is a later decision).

## Email

Sending is asynchronous. The sender writes the email with `entity.EncodeEmail` and publishes it on
the `entity.EmailQueue` topic.
For the welcome email the sender is the `send-welcome-email` event handler
(`internal/usecase/user/event`), which reacts to a registration through a notifier port,
`repo.UserNotifier`, implemented in `internal/repo/notify`, so the handler never sees the broker or
the email encoding. The message ID is `welcome-<user id>`, so a handler that runs again for the same
registration (the worker crashed before its checkpoint moved) is dropped as a duplicate. A
registration answers before its email is published, and a publish that fails is retried with the
event, so a registration never fails over its email.
The worker's `mail` handler (`v1.NewMailRoutes`) decodes it with
`entity.DecodeEmail` (both ends are this codebase, so it skips the request DTO) and sends the
`DeliverEmail` command (`internal/usecase/mail`), whose handler renders and sends.
Delivery is at-least-once, so an email can arrive twice if the worker stops between the relay
accepting it and the message being acknowledged.

In development and in the integration stack, [Mailpit](https://mailpit.axllent.org/) catches every
email: the inbox is at http://127.0.0.1:8025.
Register a user to get one.

### Templates

[`internal/repo/mailtmpl`](../internal/repo/mailtmpl) renders email from Go templates embedded in the
binary: the HTML part with `html/template`, which escapes user data, and the subject and plain-text
part with `text/template`.

- `templates/layout/` holds what every email shares: `layout.html.tmpl` (table markup, inline styles),
  `partials.html.tmpl` (`heading`, `paragraph`, `button`, `footnote`) and `layout.txt.tmpl`.
- An email is two files: `templates/<name>.html.tmpl` defining `content`, and
  `templates/<name>.txt.tmpl` defining `subject` and `text`. They execute against the `entity.Email`
  whose `Template()` returns `<name>`.
- Templates are parsed at startup, so a broken one stops the binary, as does a `MAIL_FROM` that is
  not exactly one address. A template that reads a field its
  struct lacks fails `make test`: the render test executes every email against a full and a zero-value
  fixture and compares the output with `testdata/<name>.golden.{html,txt}`.

### SMTP

[`pkg/smtp`](../pkg/smtp) is a client on the standard library's `net/smtp`: one connection per
message, STARTTLS when the server offers it, PLAIN auth when a username is set. A message goes out as
`multipart/alternative` (text, then HTML, both quoted-printable), with a Q-encoded subject that cannot
carry a header. [`internal/repo/mailer`](../internal/repo/mailer) implements `repo.Mailer` on it,
turning the `repo.RenderedEmail` that `mailtmpl` renders into an `smtp.Message`; another provider
is another implementation of that interface.

### Adding an email

1. A struct in `internal/entity/email_<name>.go` with `Template()` and `Recipient()`, and its zero
   value in `EmailTypes()` in `email.go`; `DecodeEmail` reads only the types listed there.
2. `templates/<name>.html.tmpl` and `templates/<name>.txt.tmpl`.
3. A full and a zero-value fixture in `mailtmpl_test.go`, then
   `UPDATE_GOLDEN=1 go test ./internal/repo/mailtmpl/`, and review the golden files.
4. Publish it from an event handler for the event that sends it: `body, err := entity.EncodeEmail(e)`,
   then `Publish(ctx, entity.EmailQueue, id, body)` with an ID fixed by the event.
   To keep the handler unaware of the broker, put this behind a port, as the welcome email does: a
   method on `repo.UserNotifier` in `internal/repo/user.go`, implemented in
   `internal/repo/notify`.
