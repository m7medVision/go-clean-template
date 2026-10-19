# Queue, worker and email

Background work goes through a queue in Postgres, consumed by a separate binary, `cmd/worker`. There is
no broker and no extension: the queue is one ordinary table on the stock Postgres image. Registration
uses it to send a welcome email.

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

The settings are in [configuration.md](configuration.md#worker-and-email).

## Handlers

Handlers live in [`internal/controller/worker/v1`](../internal/controller/worker/v1) and are
registered in `v1/router.go` (`w.Handle(queue, handler)`). A queue message is an inbound request like
an HTTP call, so a handler has the same shape: decode the body into a `v1/request` DTO, call a use case
with the handler's `ctx`, and return its error.

- `example.go` only logs the message. Try it with:

  ```sh
  psql "$PG_URL" -c "INSERT INTO queue_messages (queue, body) VALUES ('example', '{\"text\": \"hi\"}')"
  ```

- `mail.go` delivers email from the `mail` queue (below).

### Adding a queue

1. Handler: `internal/controller/worker/v1/<name>.go`, registered in `v1/router.go`.
2. Producer: inject `repo.QueueRepo` into the use case and call `Send(ctx, "<name>", body)`.

## Email

Sending is asynchronous. A use case writes the email with `entity.EncodeEmail` and enqueues it on
`entity.EmailQueue`. The worker's `mail` handler decodes it with `entity.DecodeEmail` (both ends are
this codebase, so it skips the request DTO) and calls the `mail` use case, which renders and sends.
Delivery is at-least-once, so an email can arrive twice if the worker stops between the relay
accepting it and the queue row being deleted.

In development and in the integration tests, [Mailpit](https://mailpit.axllent.org/) catches every
email: the inbox is at http://127.0.0.1:8025. Register a user to get one.

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
carry a header. It implements `repo.Mailer`; another provider is another implementation of that
interface.

### Adding an email

1. A struct in `internal/entity/email.go` with `Template()` and `Recipient()`, and a case for it in
   `DecodeEmail`.
2. `templates/<name>.html.tmpl` and `templates/<name>.txt.tmpl`.
3. A full and a zero-value fixture in `mailtmpl_test.go`, then
   `UPDATE_GOLDEN=1 go test ./internal/repo/mailtmpl/`, and review the golden files.
4. In the use case: `body, err := entity.EncodeEmail(e)`, then `queue.Send(ctx, entity.EmailQueue, body)`.
