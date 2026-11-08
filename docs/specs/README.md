# Specs

Two modules, built in this order. Each has its own spec and ships as its own PR.

| Module id | Responsibility | Depends on | PR | Spec |
| --- | --- | --- | --- | --- |
| `cqrs` | Every domain moves to CQRS with event sourcing: command and query buses, aggregates over an event store, projections, event handlers, the backfill of existing data | — | 1 | [cqrs.md](cqrs.md) |
| `generator` | `make new-domain` and `make new-usecase`, which render the CQRS shape from `.tmpl` files with stub handler bodies | `cqrs` | 2 | [generator.md](generator.md) |

Build order: `cqrs` → `generator`. The generator's templates copy the shapes `cqrs` settles, so
its spec is checked against the merged `cqrs` code before its plan is written.

After both merged, a structural refactor of the Go service: [untangle.md](untangle.md) (decision
log), [untangle-plan.md](untangle-plan.md), [untangle-todo.md](untangle-todo.md).

## Valkey

Three modules, one PR (approved 2026-11-08), one commit series per module, built in this order.

| Module id | Responsibility | Depends on | Spec |
| --- | --- | --- | --- |
| `valkey` | Valkey joins the stack: the `pkg/valkey` client, traced at the client level; `VALKEY_URL`, without which the app refuses to start; the compose service, `make dev`, the CI integration job and the docs | — | [valkey.md](valkey.md) |
| `exchange-cache` | `ConvertCurrency` reads a base currency's daily rates from Valkey and calls ExchangeRate-API only on a miss, until the provider's next update. Behind `repo.ExchangeWebAPI`: the use case and the transports do not change | `valkey` | [exchange-cache.md](exchange-cache.md) |
| `signin-limit` | Sign-up and sign-in attempts limited per client and per account, counted in Valkey so every app instance shares them: a `behavior.Limit` step on the buses; REST 429, gRPC `ResourceExhausted` | `valkey` | signin-limit.md (next) |

Build order: `valkey` → `exchange-cache` → `signin-limit`. The cache is the smallest real user of
the client, so it proves the client, config and CI first; the limit carries the most decisions.
Only the app uses Valkey: the worker runs neither a conversion nor a sign-in.
