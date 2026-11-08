# Implementation plan: Valkey in the stack

Spec: [valkey.md](valkey.md) · Tasks: [valkey-todo.md](valkey-todo.md) · Map: [README.md](README.md#valkey)

## Overview

The first of three modules on branch `feat/valkey`, which ship as one PR. This module makes Valkey a
service every stack runs and the app requires, with a traced client in `pkg/valkey`. Nothing reads
or writes a key yet: `exchange-cache` is the first user. Five tasks, one commit each.

## Order, and why

Every commit leaves every way of running the app working, so Valkey runs everywhere **before** the
app requires it:

```
1. pkg/valkey client          nothing calls it yet
2. Valkey in every stack      compose, make dev, CI: running, unused
3. the app requires it        feat(valkey)!: the breaking commit
4. integration test           proves the URL each stack hands out reaches Valkey
5. docs                       configuration, development, architecture, README, CLAUDE.md
```

Swapping 2 and 3 would leave a commit where `make dev` and CI fail: the app would refuse to start
with no Valkey to reach.

## Commits

| Task | Subject |
| --- | --- |
| 1 | `build(valkey): add a traced client in pkg/valkey` |
| 2 | `build: run Valkey in compose, make dev and the CI integration job` |
| 3 | `feat(valkey)!: refuse to start without VALKEY_URL`, footer `BREAKING CHANGE: the app needs VALKEY_URL` |
| 4 | `test(integration): reach Valkey with the URL the stack gives` |
| 5 | `docs: Valkey in the stack` |

`build`, `test` and `docs` release nothing; the `feat!` commit makes the release a minor bump
below 1.0 and lists itself under "Breaking changes".

## Gates

- **G1, every commit:** `make format`, `make linter-golangci`, `make test`, run one at a time with
  `GOMEMLIMIT=1GiB GOFLAGS=-p=2 GOMAXPROCS=4`.
- **G2, tasks 2 and 3:** the stack comes up: Valkey healthy, and after task 3 the app starts against
  it and stops with an error naming `valkey.New` when it is down.
- **G3, the checkpoint:** the integration suite passes, run in an isolated compose project (no host
  ports, its own project name), so it cannot remove a `make dev` session's containers.
- **G4, after the push:** CI green, the integration job included.

## Risks

| Risk | Mitigation |
| --- | --- |
| `valkeyotel` raises the OpenTelemetry versions in `go.mod` | read `make deps`' diff in task 1; if it moves otel, G1 runs the whole suite anyway, and the bump is named in the commit body |
| `valkey.NewClient` dials when it is built, so the "unreachable" unit test waits on a timeout | dial a closed port on 127.0.0.1, which is refused at once, never an unroutable address |
| The host's Postgres holds 5432 | `PG_PORT=5433` for G2 and G3 runs |
| A `make dev` session is running when G3 starts | check `tmux ls` and the ports first; G3 runs in its own compose project either way |
| CI's integration job runs on the runner, not in compose | task 2 adds the `docker run` and the readiness wait there; `.env.example`'s `VALKEY_URL` points at `localhost`, which is where that container listens |
| The worker shares compose's backend environment block | it receives `VALKEY_URL` and ignores it: the worker reads no `Valkey` section |

## Out of scope here

The `deps` field holding the client (it arrives with its first reader in `exchange-cache`),
`valkeylimiter`, the client address and `HTTP_TRUSTED_PROXIES` (`signin-limit`), Valkey metrics in
Prometheus.
