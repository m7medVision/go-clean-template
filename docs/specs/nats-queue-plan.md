# Plan: nats-queue

Spec: [nats-queue.md](nats-queue.md). Branch `feat/nats-queue`, on `feat/goose` (its drop migration
is a goose migration).

1. **`pkg/broker`.** Watermill's `pkg/nats` publisher and subscriber with JetStream on
   (auto-provisioned stream per topic, message-ID tracking, durable consumer and queue group
   `worker-<topic>`); a router whose middleware is poison → give-up log → retry → recoverer;
   `Permanent`. Unit-tested on `gochannel`; checked once against a real NATS.
2. **Producers and handlers.** `notify.Publisher` replaces `notify.Queue`; the mail controller
   takes a body and returns `broker.Permanent` where it used to drop.
3. **Wiring.** `config.NATS`, the new `config.Worker`; `RunWorker` checks `NATS_URL`, builds the
   broker, passes it to the event handlers (`deps.broker`) and the topics (`queues` on a router).
4. **Removal.** `pkg/worker`, the queue repo, `queue.sql`, the sqlc override; a goose migration drops
   the table.
5. **Infrastructure.** Compose `nats` (volume, health check), `NATS_URL` for the backend and the
   integration tests, `COMPOSE_SERVICES`, `.env.example`, CI `docker run` in the integration and
   web e2e jobs.
6. **Tests and docs.** Integration tests over NATS; every doc that described the Postgres queue.

Release: `feat(worker)!:` with the BREAKING CHANGE footer. Verification: format, lint, test,
schema round trip, then CI.
