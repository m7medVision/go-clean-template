# Tasks: untangling the Go service

Plan: [untangle-plan.md](untangle-plan.md) · Decision log: [untangle.md](untangle.md)

Gates (G1–G4) are defined in the plan. Every task: docs it touches updated in the same commit
(CLAUDE.md's "Keep the docs current" table), one Conventional Commit, the box ticked here.
Sizes: S = 1–2 files, M = 3–5, L = 6–8; a larger task is a compiler-driven mechanical sweep and says so.

---

## Phase 1: Safety net and logging

### 1. Integration health check retries a refused connection  (N)
- [x] `integration-test/helpers_test.go` retries on a connection error as well as on a non-200.

**Verify:** `go vet ./integration-test/...`. **Depends on:** none. **Size:** S

### 2. Log through `log/slog`; delete `pkg/logger`  (K)
- [x] Every logger is a `*slog.Logger`; `pkg/logger` is gone; the level comes from `LOG_LEVEL`.
- [x] No `err.Error()` used as a format; std `log` calls (`pkg/postgres`, `app/migrate.go`) use slog.
- [x] `pkg/cqrs` no longer imports a module package for logging; the 5 fake loggers are a discard handler.

**Verify:** G1. **Depends on:** none. **Size:** mechanical sweep.

### Checkpoint 1
- [x] G3 green (integration suite proves the logging swap changed nothing else).

## Phase 2: Errors and messages

### 3. Sentinels carry their kind  (A)
- [x] `entity.Error{Kind, Msg}`, `entity.KindOf(err)`; every `entity.Err*` has the kind its transports
      map it to today; `ErrRoleRequired` and its branch in `Authorize` gone.

**Verify:** G1. **Depends on:** none. **Size:** M

### 4. Each transport maps kinds once  (A)
- [x] REST `apierr`, gRPC `rpcerr`, GraphQL `gqlerr` each have one kind→status function that logs
      Internal once; every domain `problem`/`fail` and passkey's inline blocks are gone; `gqlerr` holds
      no domain sentinel.
- [x] Every REST command operation lists 409 and 503 (one helper; 501 only a generated stub can
      answer, so its op lists it); status codes and messages on the wire are unchanged.

**Verify:** G2. **Depends on:** 3. **Size:** L

### 5. Messages declare their result type; names come from the type  (D)
- [x] `cqrs.Command[R]`/`cqrs.Query[R]` markers; `cqrs.SendCommand`/`SendQuery` infer `R`; no
      `CommandName`/`QueryName` methods; log/metric names unchanged.
- [x] Unreachable bus code and the metrics `AlreadyRegistered` path removed.

**Verify:** G1. **Depends on:** none. **Size:** mechanical sweep.

### 6. `Written` leaves `pkg/cqrs`; the wait step reads a named interface  (D)
- [x] `Written` (ID, position) lives in `entity` beside the events; `Wait` uses a named interface, not an
      inline duck type; `DeliverEmail` returns no `Written`; `mail.Register` has the common shape.

**Verify:** G1. **Depends on:** 5. **Size:** M

## Phase 3: Domain rules

### 7. One page type and one paging rule  (B)
- [x] `entity.Page` (default 10, max 100) normalizes every list query; read-model contracts take it.
- [x] REST task list max 100; gRPC list limits enforced; GraphQL and REST hints match the entity.

**Verify:** G2. **Depends on:** 4, 5. **Size:** L

### 8. Task and sign-up rules in the domain  (B)
- [x] `CreateTask`, `UpdateTask`, `TransitionTask`, `RegisterUser` implement `Validate()` with the
      rules the REST tags hold today; gRPC answers `InvalidArgument` for what REST answers 422.
- [x] Unused gRPC `*validator.Validate` fields gone; a test pins REST/GraphQL hints to the entity constants.

**Verify:** G2. **Depends on:** 4. **Size:** L

### 9. Exchange amount and passkey name rules in the domain  (B)
- [x] `ConvertCurrency` validates the amount once for every transport; REST op lists the status it
      returns. Passkey names are normalized by the aggregate.

**Verify:** G2. **Depends on:** 4. **Size:** M

### 10. Read models stop re-checking the domain  (B, N)
- [x] `varchar(255)` → `text`, users' unique indexes → plain indexes, `timestamp` → `timestamptz`,
      exchange history ordered by event time; `make sqlc` clean.

**Verify:** G1, then G3 at the checkpoint. **Depends on:** 8. **Size:** M

### Checkpoint 2
- [x] G3 green.

## Phase 4: Write model and ports

### 11. `aggregate.Change`/`Create` follow from the aggregate type  (C)
- [x] Generic over the aggregate; no stream-ID, zero-value or not-found arguments at call sites.

**Verify:** G1. **Depends on:** 6. **Size:** L (mechanical)

### 12. Aggregates own a private state  (C)
- [x] Task and passkey aggregates keep a private state and snapshot; stored snapshots still load
      (same JSON or a bumped snapshot version); user and conversion aggregates lose the state and
      snapshots nothing reads; duplicate `Apply` case gone; event versions per event, same values.
- [x] Passkey ownership checked by the aggregate; the OIDC identity claim built in one place.

**Verify:** G1 (eventstore contract suite included). **Depends on:** 11. **Size:** L

### 13. Sessions and passwords  (F)
- [x] One port issues tokens (over `pkg/jwt`); every sign-in uses it; one sign-in result type;
      bcrypt hash and compare live together on `entity.User`; use cases no longer import `*jwt.Manager`.

**Verify:** G1. **Depends on:** 5. **Size:** L

### 14. OIDC adapter behind a port  (F)
- [x] `usecase/oidc/sso` moves under `internal/repo`; use cases depend on a port; its scope label and
      lifetime live in the adapter; handler tests use the port's mock.

**Verify:** G1. **Depends on:** 13. **Size:** L

### 15. WebAuthn adapter behind a port  (F)
- [x] `usecase/passkey/ceremony` moves under `internal/repo`; `Started` no longer leaks into REST;
      no state passed through callback side effects; `CeremonyTimeout` passed once. (Passkey's Deps
      and sign-in handler stay pointers: gocritic's hugeParam refuses them by value.)

**Verify:** G1. **Depends on:** 13. **Size:** L

### Checkpoint 3
- [ ] G3 green.

## Phase 5: Event store, projections, contracts

### 16. One pgx-free codec for both stores  (E)
- [ ] Codec, upcast registry (next to the events), claim rules and `SnapshotDue` in one package used
      by the Postgres store and `memstore`; `ErrClaimTaken` in `entity`; one codec built once;
      `memstore` matches Postgres (empty-save early return, decode errors returned, upcasts).

**Verify:** G1. **Depends on:** 12. **Size:** L

### 17. Projectors are values that declare what they handle  (E)
- [ ] `Projector{Name, Tables, Handles, Project}` and an event-handler value; no `Tables()` duck typing;
      the runner skips events a subscriber doesn't handle without a transaction per event; startup
      fails on an event type no projector handles or declares ignored.

**Verify:** G1. **Depends on:** 16. **Size:** L

### 18. Writes wait only on the projectors of what they wrote  (E)
- [ ] `Written` carries the event types; `Wait` waits on the projectors handling them; the app gets
      projector names without building the notifier or the queue.

**Verify:** G1. **Depends on:** 17. **Size:** M

### 19. No hand-written tracing decorators  (I)
- [x] The 9 `tracing.go` files are gone; constructors return their type; no PII in spans; CLAUDE.md's
      "three places per method" rule removed.

**Verify:** G1. **Depends on:** none. **Size:** L (deletions)

### 20. Contracts per domain; one way to join a transaction  (J)
- [ ] `repo` contracts split per domain with mocks per file; unused mocks gone; contracts import no
      `pkg/worker`/`pkg/smtp`; sentinels in each domain's entity file.
- [ ] Every read model reads through `Executor(ctx)`; `Pool` unexported; the queue maps rows itself;
      `safeIntToInt32` clamps.

**Verify:** G1. **Depends on:** 19. **Size:** L

### Checkpoint 4
- [ ] G3 green.

## Phase 6: Composition, auth, worker

### 21. Config by section; defaults in one place  (G)
- [ ] Section types exported and passed by section; no mirror structs; defaults only in `config`;
      `BaseURL` normalized once; migrate takes `cfg.PG.URL` through `pkg/postgres`; `cmd/openapi`
      needs no env; controllers don't import `config`; `resttest` sets no env.

**Verify:** G2. **Depends on:** 2. **Size:** L

### 22. One lifecycle: `Run(ctx) error`  (G, M)
- [ ] `pkg/httpserver`/`pkg/grpcserver` gone; servers, queue worker and projection runner run under
      one errgroup with `signal.NotifyContext`; no `Fatal`; WriteTimeout derived from the consistency
      timeout; `Shutdown`/`Queues()`/`Subscribers()` dead code gone.

**Verify:** G1. **Depends on:** 21. **Size:** L

### 23. One module per domain at the composition root  (G)
- [ ] `internal/app` has one file per domain building its read model, handlers, subscribers, importer
      and transport registration; app and worker iterate one list; each binary installs only what it
      uses; REST options and the duplicate route lists gone; one chi mux mounts REST and GraphQL.

**Verify:** G2. **Depends on:** 18, 21, 22. **Size:** L

### 24. One token reader; "public" declared once  (H)
- [ ] One token→principal reader and context key for all transports, `issuedAt` everywhere; REST
      enforces auth from each operation's `Security`; gRPC public methods by generated constants;
      dead caller checks gone; gRPC recovers panics and logs/measures requests.

**Verify:** G2. **Depends on:** 23. **Size:** L

### 25. Worker and queue  (M)
- [ ] The `example` queue, `worker/v1/request` and the extra router level gone; handler context
      bounded by the visibility timeout; `queue.Delete` honours its queue; `MAIL_FROM` parsed at startup.

**Verify:** G1. **Depends on:** 22, 23. **Size:** M

### 26. Transport response helpers  (N)
- [ ] gRPC timestamps through one UTC RFC 3339 formatter; one int32 clamp; the conversion's decimal
      places come from `entity`; identical REST/gRPC output types merged.

**Verify:** G2. **Depends on:** 4. **Size:** M

### 27. One file per operation per transport  (O)
- [ ] REST ops, gRPC methods and GraphQL fields of every domain live one per file (GraphQL schema
      fields per file via `extend type`); no behaviour or schema change.

**Verify:** G2 (no diff in `docs/openapi/v1.json` or the GraphQL schema). **Depends on:** 24. **Size:** mechanical.

### Checkpoint 5
- [ ] G3 green.

## Phase 7: Backfill

### 28. The backfill, isolated  (L)
- [ ] Markers in their own table; checkpoint queries lose the `_` filter for markers; one generic
      import loop with skip-on-conflict in `eventstore.Backfill`; importers export nothing for tests;
      `Rebuild` refuses while an import is pending.

**Verify:** G3. **Depends on:** 20. **Size:** L

## Phase 8: Generator

### 29. Templates and goldens follow the app  (O)
- [ ] Every template mirrors the refactored `task` domain and shared files; goldens updated and reviewed.

**Verify:** G4. **Depends on:** 1–28. **Size:** mechanical.

### 30. Fewer shared-file edits, less machinery  (O)
- [ ] new-domain edits only the app's module list and `graphql/resolver.go`; gqlgen autobind; unused
      locators (`golists.go`, `yaml.go`) and side-effect `Usecase` fields gone; new-usecase creates files.

**Verify:** G4. **Depends on:** 29. **Size:** L

### 31. The generator asks the app instead of copying it  (O)
- [ ] Regenerator list only in the Makefile, undo from `git status`; reserved packages from `go list`;
      transport paths in one place; routes read from `docs/openapi/v1.json`.

**Verify:** G4. **Depends on:** 30. **Size:** M

### 32. Generator special cases  (O)
- [ ] Swallowed errors returned; porcelain rename/quoting fixed; `AfterLastStmt` exactly once;
      test-only exports and duplicated helpers gone; per-domain exceptions that uniform domains remove gone.

**Verify:** G4. **Depends on:** 31. **Size:** M

### Checkpoint 6
- [ ] G4 green.

## Phase 9: Comments, tests, sweep

### 33. Comments  (P)
- [ ] No `// X -.` placeholders; no comment that restates the code; external facts kept.

**Verify:** G1. **Depends on:** 32. **Size:** mechanical.

### 34. Tests  (Q)
- [ ] The decision log's remove/rewrite/consolidate/fix lists done; every remaining test fails on a
      real regression.

**Verify:** G1. **Depends on:** 33. **Size:** mechanical.

### 35. Fresh-eyes sweep until nothing is worth fixing
- [ ] Fresh reviewers report no remaining special case, hidden link, needless link or surplus worth
      fixing; whatever they found is fixed.

**Verify:** G4. **Depends on:** 34.

### Checkpoint 7 (done)
- [ ] G4 green; docs current; decision log updated with what changed during the work.
