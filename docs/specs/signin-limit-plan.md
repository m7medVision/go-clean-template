# Implementation plan: sign-in and sign-up attempts limited in Valkey

Spec: [signin-limit.md](signin-limit.md) · Tasks: [signin-limit-todo.md](signin-limit-todo.md) · Map: [README.md](README.md#valkey)

## Overview

The third and last module on branch `feat/valkey`, in draft PR #52. It has ten tasks, one commit
each:
- first the client address, end to end, through HTTP, gRPC and compose;
- then the error kind on every transport;
- then the counters behind their port;
- then the step;
- then the wiring that turns it on;
- then the end-to-end test and the docs.

## Order, and why

```
 1. pkg/clientip            the rule, alone
 2. HTTP client address     TRUSTED_PROXY, httpmw.ClientAddress, HTTP logs show the client
 3. gRPC client address     the interceptor, gRPC logs show the client
 4. compose                 Caddy at a fixed address, the app trusts only it
 5. the rate-limited kind   entity errors, the three mappers, 429 in the REST docs
 6. the counters            repo.Limiter, internal/repo/limiter on valkeylimiter
 7. the step                behavior.Limit, LogIn and RegisterUser opt in
 8. turn it on              per-binary buses, AuthLimit config, the stack's raised client budget
 9. integration test        the account budget and the reset, end to end
10. docs
```

- **Tasks 1 to 4** change only which address the logs show, so they ship value before any limit
  exists.
- **Tasks 5 to 7** add code nothing calls yet. Each is tested on its own.
- **Task 8 is the one `feat` commit that turns the limit on.** It also raises the client budget
  in the integration stack and CI. Without that raise, that same commit would fail the suite: the
  suite signs up and in about 33 times from one address.

## Commits

| Task | Subject |
| --- | --- |
| 1 | `build(clientip): resolve the client behind one trusted proxy` |
| 2 | `feat(http): log the client behind TRUSTED_PROXY` |
| 3 | `feat(grpc): log the client behind TRUSTED_PROXY` |
| 4 | `build: give Caddy a fixed address in compose and trust only it` |
| 5 | `refactor(entity): a rate-limited kind and its status on every transport` |
| 6 | `build(limiter): count attempts in Valkey with valkeylimiter` |
| 7 | `refactor(behavior): a Limit step for the messages that opt in` |
| 8 | `feat(auth): limit sign-in and sign-up attempts per client and per account` |
| 9 | `test(integration): the account budget and its reset` |
| 10 | `docs: the sign-in limit and the client address` |

The `feat` commits release nothing beyond the minor version `feat(valkey)!` already makes.

## Gates

- **G1, every commit:** `make format`, `make linter-golangci`, `make test`, one at a time with
  `GOMEMLIMIT=1GiB GOFLAGS=-p=2 GOMAXPROCS=4`. Tasks 6 and 9 also run
  `go vet ./integration-test/...`.
- **G1 for the REST and web side, task 5:** `make openapi`, `make web-api` and `make web-check`.
- **G2, task 8, by hand:** `make run` against `db`, `valkey` and the worker:
  - the 21st sign-in from one client answers 429 with `Retry-After`;
  - with `valkey` stopped, sign-in works, and each attempt logs one warning;
  - restart `valkey` afterwards.
- **G3, the checkpoint:**
  - the integration suite passes in the isolated project (`gct-valkey-it`);
  - inside it, a request to Caddy with a forged `X-Forwarded-For` is logged by the app with the
    real client.
- **G4, after the push:** CI green on PR #52, the integration job included.

## Risks

| Risk | Mitigation |
| --- | --- |
| The pinned subnet (`172.30.0.0/24`) overlaps another Docker network on the host, and compose refuses to start | Docker allocates its own networks from `172.17`–`172.31` as /16s, and this host uses `172.17`–`172.20` today. The subnet is written once in `docker-compose.yml`, so moving it is one line. |
| The isolated G3 project pins the same subnet, so it can't start while the dev stack runs | before G3, `isolate.py` also rewrites the subnet and Caddy's address (to `172.31.250.0/24` and `.10`), and the app's `TRUSTED_PROXY` with them |
| Moving the buses out of the bootstrap breaks what reads `b.buses` (handler and queue registration, the servers) | task 8 builds them in `Run` and `RunWorker` before anything registers on them; `internal/app`'s tests already start both binaries' wiring |
| `valkeylimiter`'s `Close` closes the app's shared client | the adapter builds its two limiters with a `ClientBuilder` that returns the app's client, exposes no `Close`, and the app closes its client once, as today |
| `exhaustive` fails on every switch over `entity.Kind` once the kind exists | task 5 adds the kind and all three mappers in one commit |
| The integration suite exhausts the client budget | task 8 sets `AUTH_LIMIT_CLIENT_ATTEMPTS=1000` in `docker-compose-integration-test.yml` and in the CI integration job's env, in the same commit that turns the limit on |
| The CI hadolint step is still failing from the GitHub release-download outage | unrelated to this module; the retries running in the background, or your call on the workflow, settle it |
| A `make dev` session is running when G2 or G3 starts | check `tmux ls` and the ports first |

## Out of scope here

- limiting passkey or SSO sign-in;
- failing closed;
- a CAPTCHA;
- CDN headers;
- metrics for refusals.
