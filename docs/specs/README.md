# Specs

Two modules, built in this order. Each has its own spec and ships as its own PR. Both are done: `cqrs` merged 2026-11-04 (#45), `generator` 2026-11-05 (#47), released in v0.3.0.

| Module id | Responsibility | Depends on | PR | Spec |
| --- | --- | --- | --- | --- |
| `cqrs` | Every domain moves to CQRS with event sourcing: command and query buses, aggregates over an event store, projections, event handlers, the backfill of existing data | — | 1 | [cqrs.md](cqrs.md) |
| `generator` | `make new-domain` and `make new-usecase`, which render the CQRS shape from `.tmpl` files with stub handler bodies | `cqrs` | 2 | [generator.md](generator.md) |

Build order: `cqrs` → `generator`. The generator's templates copy the shapes `cqrs` settles, so
its spec is checked against the merged `cqrs` code before its plan is written.

After both merged, a structural refactor of the Go service (done: merged 2026-11-07, #51, v0.3.1): [untangle.md](untangle.md) (decision
log), [untangle-plan.md](untangle-plan.md), [untangle-todo.md](untangle-todo.md).

## Valkey

Three modules, one PR (approved 2026-11-08; done: merged 2026-11-08, #52, v0.4.0), one commit series per module, built in this order.

| Module id | Responsibility | Depends on | Spec |
| --- | --- | --- | --- |
| `valkey` | Valkey joins the stack: the `pkg/valkey` client, traced at the client level; `VALKEY_URL`, without which the app refuses to start; the compose service, `make dev`, the CI integration job and the docs | — | [valkey.md](valkey.md) |
| `exchange-cache` | `ConvertCurrency` reads a base currency's daily rates from Valkey and calls ExchangeRate-API only on a miss, until the provider's next update. Behind `repo.ExchangeWebAPI`: the use case and the transports do not change | `valkey` | [exchange-cache.md](exchange-cache.md) |
| `signin-limit` | Sign-up and sign-in attempts limited per client and per account, counted in Valkey so every app instance shares them: a `behavior.Limit` step on the buses; REST 429, gRPC `ResourceExhausted` | `valkey` | [signin-limit.md](signin-limit.md) |

Build order: `valkey` → `exchange-cache` → `signin-limit`. The cache is the smallest real user of
the client, so it proves the client, config and CI first; the limit carries the most decisions.
Only the app uses Valkey: the worker runs neither a conversion nor a sign-in.

## MongoDB

Two modules, one PR, built in this order, after the Valkey PR merges. Approved 2026-11-09.

| Module id | Responsibility | Depends on | Spec |
| --- | --- | --- | --- |
| `mongo` | Mongo joins the stack: the `pkg/mongo` client, traced at the client level; `MONGO_URL`, without which the app refuses to start; compose, `make dev`, the CI integration job and the docs | — | [mongo.md](mongo.md) |
| `task-attributes` | Typed task attributes stored in Mongo, written directly (no events), merged into every task read, patched with merge semantics on REST, gRPC, GraphQL and the web | `mongo` | [task-attributes.md](task-attributes.md) |

Build order: `mongo` → `task-attributes`. Only the app uses Mongo.

## Migrations

One module, built before `nats-queue`. Spec approved 2026-11-08; done: merged 2026-11-09 with #56, v0.6.0.

| Module id | Responsibility | Depends on | Spec |
| --- | --- | --- | --- |
| `goose` | The schema moves from Atlas's declarative HCL to versioned goose SQL migrations, applied at app start under an advisory lock; sqlc reads the migrations; the generator writes migrations; Atlas removed | — | [goose.md](goose.md) |

## Observability and messaging

Four modules, after the MongoDB modules. `nats-queue` is done (merged 2026-11-09, #56, v0.6.0; the outbox turned out not to be needed); the
others written 2026-11-08, awaiting review; each to be grilled again before approval.

| Module id | Responsibility | Depends on | Spec |
| --- | --- | --- | --- |
| `nats-queue` | The queue moves to NATS JetStream through Watermill: router in the worker (NATS is the worker's only), outbox design pending, retry with backoff, poison topic, recover; `queue_messages` and `pkg/worker` removed | `goose` | [nats-queue.md](nats-queue.md) |
| `glitchtip` | Unexpected backend errors to GlitchTip through one `slog` wrapper (the Sentry protocol, no strategy interface), with breadcrumbs per request on an attribute allowlist; a stuck projection reported once per position; `SENTRY_DSN`, a project per binary. Approved 2026-11-10; built, awaiting review | — | [glitchtip.md](glitchtip.md) |
| `posthog` | Optional analytics: a typed event catalogue, seven launch events through the outbox to PostHog, web page views, identify and masked replay | `nats-queue`, `task-attributes` | [posthog.md](posthog.md) |
| `feature-flags` | OpenFeature; GO Feature Flag on `flags.yaml` in development, code defaults in production; `GetFlags` on REST and GraphQL; the `task-attributes` flag | `task-attributes` | [feature-flags.md](feature-flags.md) |

Build order: `goose` (above) → `nats-queue` → `glitchtip` → `posthog` → `feature-flags`.

## Web

Six modules for `web/`; capability map approved 2026-11-08. All six are done: merged 2026-11-08 (#53), v0.5.0. Specs: [web-boundaries.md](web-boundaries.md)
(done), [plan](web-boundaries-plan.md), [tasks](web-boundaries-todo.md); [web-errors.md](web-errors.md) (done), [plan](web-errors-plan.md), [tasks](web-errors-todo.md); [web-authz.md](web-authz.md), [web-tests.md](web-tests.md), [web-e2e.md](web-e2e.md) and [web-error-tracking.md](web-error-tracking.md) (done; plans and tasks beside each). See
[web.md](web.md).
