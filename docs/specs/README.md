# Specs

Two modules, built in this order. Each has its own spec and ships as its own PR.

| Module id | Responsibility | Depends on | PR | Spec |
| --- | --- | --- | --- | --- |
| `cqrs` | Every domain moves to CQRS with event sourcing: command and query buses, aggregates over an event store, projections, event handlers, the backfill of existing data | — | 1 | [cqrs.md](cqrs.md) |
| `generator` | `make new-domain` and `make new-usecase`, which render the CQRS shape from `.tmpl` files with stub handler bodies | `cqrs` | 2 | [generator.md](generator.md) |

Build order: `cqrs` → `generator`. The generator's templates copy the shapes `cqrs` settles, so
its spec is checked against the merged `cqrs` code before its plan is written.
