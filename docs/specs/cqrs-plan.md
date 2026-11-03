# Implementation Plan: full CQRS with event sourcing

Spec: [cqrs.md](cqrs.md) · Tasks: [cqrs-todo.md](cqrs-todo.md) · Module `cqrs`, PR 1 of 2

## Overview

Build the CQRS machinery first and prove the event store against Postgres before any domain
depends on it. Then convert the domains one at a time. Task goes first and sets the pattern; user,
exchange, passkey, OIDC and mail copy it. Every task leaves `make format`, `make linter-golangci`
and `make test` green. Every domain checkpoint also leaves the integration tests green and the API
documents unchanged.

## Architecture decisions made while planning

1. **Each domain moves in four steps, with no window where writes split.**
   - Build the domain's aggregate, read model, projector and handlers beside the old use case, and
     test them.
   - **Switch:** the old use case's methods call the buses. REST, gRPC and GraphQL move to event
     sourcing in that one change. A domain is never half written through events and half through
     direct table writes.
   - Move each transport onto the buses directly.
   - Delete the old use case.
2. **During the migration, the routers take the buses and the use cases not yet converted.** Each
   domain's last task removes its use-case parameter, and the first domain's transport tasks add
   the bus parameters.
3. **The backfill is per domain** (spec updated). Each domain has its own marker, and `*Imported`
   events are projected as upserts. A domain's existing rows get their streams in the same task as
   its switch. No checkpoint is ever moved by hand, so the backfill can run against a live
   database.
4. **One contract test suite** (`internal/repo/eventstoretest`) runs against the in-memory store in
   `make test` and against Postgres in the integration tests. That way the in-memory store that
   command-handler tests rely on can't drift from the real one.
5. **`internal/repo/repomock` arrives in the foundation.** Handler tests live in sub-packages and
   can't import the `usecase_test` mocks. The old mocks stay until the last use case is gone.
6. **A generic load–change–save helper** is settled in task 17 and reused by every command handler.
   Twelve handlers of the same shape would otherwise trip `dupl` (threshold 100).
7. **Error mapping:** the two new failures that any command can produce,
   `entity.ErrConcurrencyConflict` and `entity.ErrReadModelBehind`, are mapped once per transport
   in a shared package. REST `v1/apierr`, gRPC `v1/rpcerr` and the existing GraphQL `gqlerr` follow
   the controller-layout rule for shared packages. Domain errors stay mapped inline, as today.
8. **Commits, made only when you ask:**
   - event store (tasks 1–8);
   - runner and wiring (tasks 9–14);
   - task (15–25);
   - user (26–32);
   - exchange (33–38);
   - passkey (39–45);
   - OIDC and mail (46–48);
   - backfill test and docs (49–51).

## Dependency graph

```
pkg/cqrs buses (1) ─ steps (2)          pkg/postgres tx (3)        entity base (4)
        │                                     │                          │
        │                       schema + queries (5) ─ contract, codec (6) ─ suite + memstore (7)
        │                                                     │
        │                                         Postgres event store (8)  ◀── checkpoint A
        │                                                     │
        ├── app steps (10) ── sentinel mapping (11)    projection runner (9)
        │                                                     │
        └──────── config (12) ── wiring (13) ── backfill framework (14)  ◀── checkpoint B
                                         │
           task 15–25 (pattern) ◀── checkpoint C, your review
                                         │
           user 26–32 ── exchange 33–38 ── passkey 39–45 ── OIDC + mail 46–48
                                         │
           backfill test (49) ── docs (50, 51) ── final verification (52)
```

The OIDC tasks need user (26–30), because the first SSO sign-in creates a `UserAggregate`.

## Task list

### Phase 1: Foundation (no behaviour change)
- [x] 1. Command and query buses
- [x] 2. Trace, log and metrics steps
- [x] 3. Transaction carried in the context
- [x] 4. Event-sourcing base types
- [x] 5. Event-store tables and queries
- [x] 6. EventStore contract, codec, upcasters, repomock
- [x] 7. Contract suite and in-memory store
- [x] 8. Postgres event store

### Checkpoint A: the event store
- [x] The contract suite passes on memstore (`make test`) and on Postgres (`make compose-up-integration-test`)

### Phase 1 continued
- [x] 9. Projection runner
- [x] 10. App steps: wait, retry, authorize, validate
- [x] 11. Shared mapping of the two new sentinels
- [x] 12. Config: the four env vars
- [x] 13. Wiring: buses, worker runner, rebuild command
- [x] 14. Backfill framework

### Checkpoint B: foundation
- [x] `make pre-commit` and the integration tests pass; API documents unchanged
- [ ] **Your review** of the bus, event store and runner APIs before any domain uses them

### Phase 2: Task, the pattern
- [x] 15. TaskAggregate and its events
- [x] 16. Task read model and projector
- [x] 17. CreateTask and UpdateTask, and the load–change–save helper
- [x] 18. TransitionTask and DeleteTask
- [x] 19. GetTask, ListTasks, ListAllTasks
- [x] 20. Task registration, importer and wiring
- [x] 21. Switch: task writes go through the command bus
- [x] 22. REST task on the buses
- [x] 23. gRPC task on the buses
- [x] 24. GraphQL task on the buses
- [x] 25. Remove the old task use case; CQRS integration tests

### Checkpoint C: the pattern
- [x] All gates pass; tasks behave in the web client exactly as before
- [ ] **Your review** of the task domain, which every later domain copies

### Phase 3: User and the welcome email
- [x] 26. UserAggregate and its events
- [x] 27. User read model and projector
- [x] 28. RegisterUser, LogIn, GetUser
- [x] 29. SendWelcomeEmail event handler
- [x] 30. User registration, importer and switch
- [x] 31. REST and gRPC user on the buses
- [x] 32. Remove the old user use case

### Checkpoint D: user
- [x] All gates pass; registration still sends exactly one welcome email

### Phase 4: Exchange
- [x] 33. ConversionAggregate and its events
- [x] 34. Exchange read model and projector
- [x] 35. ConvertCurrency, GetExchangeHistory, GetConversion
- [x] 36. Exchange registration, importer and switch
- [x] 37. REST, gRPC and GraphQL exchange on the buses
- [x] 38. Remove the old exchange use case; update `docs/request-flow.md`

### Checkpoint E: exchange
- [x] All gates pass

### Phase 5: Passkey
- [x] 39. PasskeyAggregate and its events
- [x] 40. Passkey read model and projector
- [x] 41. FinishPasskeyRegistration and RenamePasskey
- [x] 42. DeletePasskey and FinishPasskeyLogin
- [x] 43. Passkey queries
- [x] 44. Passkey registration, importer and switch
- [x] 45. REST passkey on the buses; remove the old use case

### Checkpoint F: passkey
- [x] All gates pass, including the passkey integration tests

### Phase 6: OIDC and mail
- [x] 46. FinishOIDCLogin, BeginOIDCLogin, GetOIDCStatus
- [x] 47. OIDC registration, switch, REST on the buses, remove the old use case
- [x] 48. DeliverEmail; remove `usecase/contracts.go`

### Checkpoint G: every domain
- [x] No `internal/usecase/*/tracing.go`, no `usecase/contracts.go`, no `usecase_test` mocks

### Phase 7: Finish
- [ ] 49. Backfill integration test
- [ ] 50. Reference docs
- [ ] 51. CLAUDE.md
- [ ] 52. Final verification and the PR

### Checkpoint H: complete
- [ ] Every success criterion in the spec holds; CI is green

## Risks and mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| An ordering or locking bug in the event store loses or skips events | High | The store and its contract suite come first, run on both stores (tasks 7, 8), and gate everything at checkpoint A. The advisory lock plus a unique `(stream_id, stream_version)` |
| The backfill loses or duplicates data | High | No table is dropped; one transaction and one marker per domain; upserts; an integration test (49); a `pg_dump` of the dev database to the scratchpad before the local check in task 52 |
| A read straight after a write is stale, so the web list doesn't update | High | The wait step (10); read-after-write integration tests on each transport (25 and each domain's last task); a web client check at every switch |
| The OpenAPI document, protos or GraphQL schema drift | Medium | Read shapes stay untouched; `make openapi web-api web-graphql` must give no diff at every checkpoint |
| `dupl` or `funlen` on many handlers of the same shape | Medium | The shared helper from task 17 |
| The PR is too big to review | Medium | One commit per domain; your review at checkpoints B and C |
| `make compose-up-integration-test` collides with a running `make dev` | Low | Stop `make dev` first, or use the isolated compose recipe |
| Open question 1 is still unanswered | Medium | Built with the spec's default (503). Only task 11's mapping changes if you choose the other answer |
| Passkey streams grow by one event per login | Low | Snapshots (task 8), covered by the contract suite |

## Parallel work

The domains share the event registry, `repo/contracts.go`, the app wiring and the routers, so they
go one after another. Inside a domain, the read model and the commands are independent once the
aggregate exists. The docs (50, 51) can be drafted during phase 6.

## Open questions

1. *(from the spec)* **When the wait times out, should the write answer 503 or answer from the
   aggregate?** The plan uses 503; it's needed before task 11.
2. *(from the spec)* **`refactor` or `refactor!`?** This only matters when the PR is created, in
   task 52.
