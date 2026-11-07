# Implementation plan: untangling the Go service

Decision log: [untangle.md](untangle.md) · Tasks: [untangle-todo.md](untangle-todo.md)

## Overview

A structural refactor of the Go code (`cmd/`, `config/`, `internal/`, `pkg/`, `integration-test/`,
`tools/gen`) so that pieces that change together live together, pieces that change for different
reasons are not tied, and everything depends on what changes less. Behaviour stays the same apart
from the bugs listed in the decision log. `web/` changes only where a generated file follows a Go
change. Branch: `refactor/untangle`, one commit per task.

## Architecture decisions

- **Error kinds on sentinels.** `entity.Error{Kind, Msg}`; each transport maps a kind to its status
  in one function. Domain folders stop mapping errors. (A)
- **Domain rules in `Validate()`**, with one `entity.Page`. Transport schema hints stay, pinned to the
  entity constants by a test. Read-model columns stop re-checking. (B)
- **Aggregates own a private state** with its own snapshot; `entity.Task`/`User`/`Passkey` are only
  the read shape. `aggregate.Change` is generic over the aggregate type. (C)
- **Messages carry their result type** (`cqrs.Command[R]`, `cqrs.Query[R]`, inferred by
  `cqrs.Send`); names come from the Go type. `Written` moves out of `pkg/cqrs`. (D)
- **Projectors are values** that declare the event types they handle; the wait step waits only on
  those. One pgx-free codec, shared by the Postgres store and `memstore`. (E)
- **Protocol adapters (OIDC, WebAuthn) and token issuing behind ports** in `internal/repo`. (F)
- **One module per domain in `internal/app`**, one bootstrap, `Run(ctx) error` for every long-running
  part, config passed by section, one chi mux that mounts REST and GraphQL. (G)
- **One token reader** for all transports; REST enforces auth from each operation's `Security`. (H)
- **No hand-written tracing decorators.** (I) **Contracts per domain file.** (J) **slog.** (K)
- **Backfill kept, isolated** in its own markers table. (L)
- **Generator follows**: fewer shared-file edits, less machinery. (O)

## Gates

- **G1:** `make format`, `make linter-golangci`, `make test`, and `make gen-check`.
- **G2:** G1, then `make openapi` and `make web-api` / `make web-graphql` give no unexpected diff, and
  `make web-check` passes (when a Huma struct, GraphQL schema or OpenAPI output changes).
- **G3:** G2, then `make compose-up-integration-test` (stop `make dev` first; `PG_PORT=5433` because
  the host Postgres holds 5432).
- **G4:** G3, then `make gen-e2e`. Only from phase 8 on: until then the templates still mirror the
  old `task` domain and gen-e2e is expected to fail on this branch.

## Risks and mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Stored data (events, snapshots, claims) no longer decodes after a type move | High | Event JSON and type names unchanged; snapshot struct keeps the same JSON or its version is bumped so old snapshots are ignored; the eventstore contract suite and the integration suite run at every checkpoint |
| A public API changes by accident (paths, status codes, JSON, proto, GraphQL) | High | `make openapi` diff reviewed at G2; REST/gRPC/GraphQL tests stay; integration suite at checkpoints |
| Large mechanical sweeps (Send calls, logger) hide a behaviour change | Medium | One sweep per task, compiler-driven, tests before and after |
| The generator drifts while the app changes | Medium | Templates move in one step (phase 8) and gen-e2e becomes a gate from there |
| Context compaction mid-task loses state | Medium | Task list ticked after every task; one commit per task; decision log in the repo |

## Open questions

None blocking. Defaults taken at approval are recorded in the decision log under
"Decisions taken when the plan was approved".
