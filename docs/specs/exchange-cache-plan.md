# Implementation plan: the exchange rates cached in Valkey

Spec: [exchange-cache.md](exchange-cache.md) · Tasks: [exchange-cache-todo.md](exchange-cache-todo.md) · Map: [README.md](README.md#valkey)

## Overview

The second of three modules on branch `feat/valkey`, in draft PR #52. It has four tasks, one commit
each. The `pkg/valkey` cache comes first, then the adapter that uses it together with its wiring,
then the end-to-end proof, then the docs.

## Order, and why

```
1. pkg/valkey.Cache           nothing calls it yet; proven against the stack's Valkey
2. the adapter reads it       webapi.Cache, New(baseURL, cache), one deadline, and the wiring
3. integration test           a seeded body proves the app reads Valkey before the provider
4. docs                       request-flow, architecture, CLAUDE.md
```

The new port, the constructor's new argument and the wiring all go in task 2's one commit. Splitting
them would leave a commit where `internal/app` doesn't build, because `webapi.New` takes a cache
that nothing passes yet.

## Commits

| Task | Subject |
| --- | --- |
| 1 | `build(valkey): add a cache with an absolute expiry to pkg/valkey` |
| 2 | `perf(exchange): read the daily rates from Valkey until the provider's next update` |
| 3 | `test(integration): a conversion served from the Valkey cache` |
| 4 | `docs: the exchange rates cached in Valkey` |

`build`, `perf`, `test` and `docs` release nothing. The PR's release is still the `feat(valkey)!`
commit's.

## Gates

The valkey plan's gates, applied to this module:

- **G1, every commit:** `make format`, `make linter-golangci`, `make test`, run one at a time with
  `GOMEMLIMIT=1GiB GOFLAGS=-p=2 GOMAXPROCS=4`. Tasks 1 and 3 also run
  `go vet ./integration-test/...`.
- **G2, task 2:** against `db`, `valkey` and the worker, with the real provider:
  - `make run` starts;
  - one conversion from OMR leaves `exchange:latest:OMR` in Valkey, with a `PEXPIRETIME` equal to
    the body's `time_next_update_unix` × 1000;
  - with `valkey` stopped (`docker compose stop valkey`), a conversion still answers 200;
  - start `valkey` again afterwards.
- **G3, the checkpoint:** the integration suite passes in an isolated compose project
  (`gct-valkey-it`, no host ports), so it can't touch a `make dev` session.
- **G4, after the push:** CI green on PR #52, the integration job included.

## Risks

| Risk | Mitigation |
| --- | --- |
| A deadline derived from the caller's `ctx` makes a slow provider look like a caller that went away, so it answers 500 instead of 502 | copy `sso`: `context.WithTimeoutCause(ctx, timeout, errTimedOut)`, and `unavailable` checks the cause. A test with a provider that never answers, under a short timeout set through `export_test.go` (`SetTimeout`, as `sso` has), expects `ErrRatesUnavailable` |
| The recorded fixture's next update has passed, so neither the integration stack nor the existing unit cases ever store a body | the unit tests that store build their body with a next update an hour ahead; the end-to-end test seeds the key itself |
| A test of a hung cache waits `_cacheTimeout` (500 ms) | it runs with `t.Parallel()`, so it adds 500 ms to the package, not per test |
| `NewCache(nil)` in `internal/app`'s module tests, whose `deps` has no client | `NewCache` only stores the client, and those tests never convert, so nothing calls it |
| The seeded USD key leaks into another test | no other test converts from USD (checked); the test deletes the key in `t.Cleanup` |
| `make run` against the real provider needs the network for G2 | if it's offline, G2 runs against the exchange stub instead. The stub's fixture is never stored, so `PEXPIRETIME` is checked through the unit tests and G3 only |
| A `make dev` session is running when G2 or G3 starts | check `tmux ls` and the ports first; G3 runs in its own compose project either way |

## Out of scope here

Everything the spec's "Decided by default" item 4 lists:
- caching an unknown base;
- stampede protection;
- an in-process cache;
- metrics for hits and misses.

Also anything for `signin-limit`.
