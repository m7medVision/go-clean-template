# Queue and worker

The worker, `cmd/worker`, is a separate binary with two jobs. It runs the **projections**: the
projectors that keep the read models up to date from the event store, and the event handlers that
react to events. And it consumes a **queue** in Postgres. There is no broker and no extension: the
queue is one ordinary table on the stock Postgres image. The welcome email after a registration
uses both: an event handler enqueues it, a queue handler delivers it.

The app needs the worker: a write waits for the projectors before it answers
([cqrs.md](cqrs.md#reads-after-writes)).

## Projections

The worker starts one loop per subscriber, the projectors in `projectors` and the event handlers in
`eventHandlers` (`internal/app/buses.go`), next to
the queue consumer, and stops them on the same SIGTERM. How a loop reads, commits and wakes is in
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

The table is `queue_messages` ([schema](../internal/app/schema/queue_messages.hcl)), read and written
through [`internal/repo/persistent/queue`](../internal/repo/persistent/queue) (`Send`, `Read`,
`Delete`). One table holds every queue, told apart by its `queue` column, so a queue exists as soon
as something is sent to it.

[`pkg/worker`](../pkg/worker) polls the registered queues and calls one handler per message:

- **Claiming.** A read takes up to `WORKER_BATCH_SIZE` due messages with `FOR UPDATE SKIP LOCKED` and,
  in the same statement, hides them until `now() + WORKER_VISIBILITY_TIMEOUT`. Several workers can
  run side by side without getting the same message at the same time.
- **At-least-once.** A message is deleted only after its handler returns `nil`. If the handler fails,
  the message comes back once its visibility timeout expires, so handlers must be idempotent.
  `worker.Message.ReadCount` tells a handler how many times it has been delivered. A body that can
  never decode should be logged and acknowledged, or it redelivers forever.
- **Shutdown.** On SIGTERM it stops reading, waits up to `WORKER_SHUTDOWN_TIMEOUT` for in-flight
  handlers, then cancels their context.
- **Startup.** The app applies the schema, not the worker. A worker that starts first logs
  `relation "queue_messages" does not exist` until the app has run, then carries on.

The settings are in [configuration.md](configuration.md#worker).

## Handlers

Handlers live in [`internal/controller/worker/v1`](../internal/controller/worker/v1) and are
registered in `v1/router.go` (`w.Handle(queue, handler)`). A queue message is an inbound request like
an HTTP call, so a handler has the same shape: decode the body into a `v1/request` DTO, send a command
on the command bus with the handler's `ctx`, and return its error.

- `example.go` only logs the message. Try it with:

  ```sh
  psql "$PG_URL" -c "INSERT INTO queue_messages (queue, body) VALUES ('example', '{\"text\": \"hi\"}')"
  ```

- `mail.go` delivers email from the `mail` queue (below).

### Adding a queue

1. Handler: `internal/controller/worker/v1/<name>.go`, registered in `v1/router.go`.
2. Producer: give the command or event handler a port whose implementation calls the queue's
   `Send(ctx, "<name>", body)` (`*queue.Repo`, through a small interface of its own, as
   `notify.Queue` does). `Send` joins the transaction in `ctx`, so from an event handler the message
   commits with the handler's checkpoint, exactly once.

## Email

Sending is asynchronous. The sender writes the email with `entity.EncodeEmail` and enqueues it on
`entity.EmailQueue`.
For the welcome email the sender is the `send-welcome-email` event handler
(`internal/usecase/user/event`), which reacts to a registration through a notifier port,
`repo.UserNotifier`, implemented in `internal/repo/notify`, so the handler never sees the queue or
the email encoding. A registration answers before its email is enqueued, and an enqueue that fails
is retried with the event, so a registration never fails over its email.
The worker's `mail` handler (registered by `workerctl.WithMail`) decodes it with
`entity.DecodeEmail` (both ends are this codebase, so it skips the request DTO) and sends the
`DeliverEmail` command (`internal/usecase/mail`), whose handler renders and sends.
Delivery is at-least-once, so an email can arrive twice if the worker stops between the relay
accepting it and the queue row being deleted.

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
- Templates are parsed at startup, so a broken one stops the binary. A template that reads a field its
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
4. Enqueue it from an event handler for the event that sends it: `body, err := entity.EncodeEmail(e)`,
   then `Send(ctx, entity.EmailQueue, body)` on the queue.
   To keep the handler unaware of the queue, put this behind a port, as the welcome email does: a
   method on `repo.UserNotifier` in `internal/repo/user.go`, implemented in
   `internal/repo/notify`.
