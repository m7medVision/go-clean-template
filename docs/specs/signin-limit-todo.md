# Tasks: sign-in and sign-up attempts limited in Valkey

Plan: [signin-limit-plan.md](signin-limit-plan.md) · Spec: [signin-limit.md](signin-limit.md)

Gates (G1–G4) are defined in the plan. Every task gets one Conventional Commit and its boxes ticked
here, and adds no comments to new code unless one is important. Sizes: S = 1–2 files, M = 3–5,
L = 6–8.

---

### 1. The rule, in `pkg/clientip`
- [x] `Resolve(peer netip.Addr, forwardedFor []string, proxy netip.Addr) netip.Addr`, as in the
      spec: the peer unless it is the proxy; from the proxy, the rightmost `X-Forwarded-For` entry
      that parses, else the peer; IPv4-mapped addresses unmapped, zones dropped.
- [x] Table-driven tests:
  - [x] no proxy configured;
  - [x] a direct client with and without a header;
  - [x] a forged header from a non-proxy;
  - [x] through the proxy;
  - [x] several header lines and comma lists (the rightmost wins);
  - [x] an unparsable rightmost entry;
  - [x] IPv4-mapped IPv6;
  - [x] an IPv6 client.

**Files:** `pkg/clientip/clientip.go`, `clientip_test.go`. **Verify:** G1. **Depends on:** none.
**Size:** S

### 2. The client address on HTTP
- [x] `config.HTTP.TrustedProxy` (`TRUSTED_PROXY`, no default, no `required`); `.env.example`
      gains an empty `TRUSTED_PROXY=`.
- [x] `internal/app` parses it once with `netip.ParseAddr` (empty means no proxy); a bad address
      stops `Run` with a wrapped error.
- [x] `entity.ContextWithClient(ctx, netip.Addr)` and
      `entity.ClientFromContext(ctx) (netip.Addr, bool)` in `internal/entity/client.go`, with a
      test.
- [x] `httpmw.ClientAddress(proxy netip.Addr)`: `clientip.Resolve(RemoteAddr,
      r.Header.Values("X-Forwarded-For"), proxy)` into the context. It is the first middleware in
      `internal/app/http.go`.
- [x] `httpmw.Logger` and `httpmw.Recovery` log `entity.ClientFromContext`, falling back to the
      remote address when it is missing.
- [x] Tests: a request from the proxy with a header, a forged header from elsewhere, no proxy
      configured; the logger's `client` field shows the resolved address.

**Files:** `config/config.go`, `.env.example`, `internal/entity/client.go` (+ test),
`internal/controller/httpmw/client.go` (+ test), `logger.go`, `recovery.go`,
`internal/app/http.go` (and `app.go` for the parse). **Verify:** G1. **Depends on:** 1.
**Size:** L

### 3. The client address on gRPC
- [x] An interceptor in `internal/controller/grpc/middleware/client.go`:
      `clientip.Resolve(peer, metadata "x-forwarded-for", proxy)` into the context. It goes first
      in the unary chain and the stream chain (`internal/app/grpc.go`).
- [x] The gRPC logger and recovery log `entity.ClientFromContext`, falling back to the peer.
- [x] Tests over `bufconn`: metadata from the proxy, forged metadata from elsewhere.

**Files:** `internal/controller/grpc/middleware/client.go` (+ test), `logger.go`, `recovery.go`,
`internal/app/grpc.go`. **Verify:** G1. **Depends on:** 2. **Size:** M

### 4. Caddy at a fixed address
- [x] `docker-compose.yml`:
  - [x] `app_network` gets `ipam` with subnet `172.30.0.0/24`;
  - [x] `caddy` gets `ipv4_address: 172.30.0.10`;
  - [x] the backend environment gets `TRUSTED_PROXY: "172.30.0.10"`.
- [x] `docker compose config` renders.
- [ ] The scratchpad's `isolate.py` learns to rewrite the subnet, the address and `TRUSTED_PROXY`
      for G3.

**Files:** `docker-compose.yml`. **Verify:** G1; `docker compose -f docker-compose.yml -f
docker-compose-integration-test.yml config` renders. **Depends on:** 2. **Size:** S

### 5. The rate-limited kind on every transport
- [x] `entity.KindRateLimited`; `ErrTooManyAttempts` (`"too many attempts"`); `RetryAfterError`
      (wraps `ErrTooManyAttempts`, carries `RetryAfter time.Duration`) and its constructor;
      `Allowance{Allowed bool; RetryAfter time.Duration}`; `Limits` (the client rule and the
      account rule: a count and a window each).
- [x] `apierr.Problem`: 429 through `huma.ErrorWithHeaders`, `Retry-After` in whole seconds,
      rounded up, never below 1.
- [x] `rpcerr.Status`: `codes.ResourceExhausted` with an `errdetails.RetryInfo`.
- [x] `gqlerr.Map`: `RATE_LIMITED`.
- [x] The REST `register` and `login` operations list 429; `make openapi`, `make web-api`,
      `make web-check`, and commit their diffs.
- [x] Tests for each mapper, and for `RetryAfterError` with `errors.Is` and `errors.As`.

**Files:** `internal/entity/errors.go`, `internal/entity/limit.go` (+ test), the three mappers
(+ tests), `internal/controller/restapi/v1/user/register.go`, `login.go`, `docs/openapi/v1.json`,
`web/src/api/schema.gen.ts`. **Verify:** G1; `make web-check`. **Depends on:** none. **Size:** L

### 6. The counters
- [x] `repo.Limiter` in `internal/repo/contracts.go` (`TakeClient`, `CheckAccount`,
      `TakeAccountFailure`, `ClearAccount`), added to its `go:generate` list; `make mock`.
- [x] `internal/repo/limiter`: `New(client vk.Client, limits entity.Limits) (repo.Limiter,
      error)`, two `valkeylimiter` clients built with a `ClientBuilder` returning `client`. Their
      key prefixes are `limit:client` and `limit:account`, and they are never closed.
- [x] Identifiers:
  - [x] client: `<scope>:<addr>`, an IPv6 client by its `/64`;
  - [x] account: `<scope>:<hex sha256 of the lower-cased, trimmed email>`.

      Unit-tested through `export_test.go`.
- [x] `valkeylimiter` v1.0.79 in `go.mod`; read `make deps`' diff.
- [x] `integration-test/limit_test.go`, against the stack's Valkey under scope `_test:<uuid>`:
  - [x] a client budget of 2 refuses the 3rd take, with a `RetryAfter`;
  - [x] after `ClearAccount`, failures count from zero.

**Files:** `internal/repo/contracts.go`, `internal/repo/repomock/contracts.go`,
`internal/repo/limiter/limiter.go` (+ `export_test.go`, test), `go.mod`, `go.sum`,
`integration-test/limit_test.go`. **Verify:** G1, `go vet ./integration-test/...`.
**Depends on:** 5. **Size:** L

### 7. The step
- [x] `behavior.Limited` (`LimitScope() string`, `LimitAccount() string`) and
      `behavior.Limit(limiter repo.Limiter, l *slog.Logger) cqrs.Step`, as in the spec's five
      steps. Its two comments: the check-then-count race, and why a limiter error lets the attempt
      through.
- [x] `LogIn` returns `"sign-in"` and its email; `RegisterUser` returns `"sign-up"` and `""`.
- [x] Tests against `repomock.MockLimiter`:
  - [x] a spent client budget refuses before the handler runs;
  - [x] a spent account budget refuses;
  - [x] `ErrInvalidCredentials` counts a failure;
  - [x] success clears the account;
  - [x] no client in the context skips the client budget;
  - [x] a message that isn't `Limited` passes;
  - [x] a limiter error lets it through and logs one warning.

**Files:** `internal/usecase/behavior/behavior.go`, `behavior_test.go`,
`internal/usecase/user/query/log_in.go`, `internal/usecase/user/command/register_user.go`.
**Verify:** G1. **Depends on:** 5, 6. **Size:** M

### 8. Turn it on
- [x] `config.AuthLimit`: `AUTH_LIMIT_CLIENT_ATTEMPTS` (20), `AUTH_LIMIT_ACCOUNT_FAILURES` (5),
      `AUTH_LIMIT_WINDOW` (15m); the same in `.env.example`.
- [x] `newBootstrap` no longer builds the buses. `newBuses` takes extra steps and puts them after
      `Stamp`. `Run` builds the app's buses after `valkey.New`, with
      `behavior.Limit(limiter.New(client, limits), l)`; `RunWorker` builds the worker's without it.
- [x] `internal/app` tests: the app's buses carry the step and the worker's don't, and both start
      as before.
- [x] `docker-compose-integration-test.yml` and the CI integration job set
      `AUTH_LIMIT_CLIENT_ATTEMPTS=1000`.

**Files:** `config/config.go`, `.env.example`, `internal/app/bootstrap.go`, `buses.go`, `app.go`,
`worker.go` (+ their tests), `docker-compose-integration-test.yml`, `.github/workflows/ci.yml`.
**Verify:** G1; G2. **Depends on:** 7. **Size:** L

### 9. The end-to-end proof
- [x] `integration-test/limit_test.go`:
  - [x] register, 5 wrong passwords (401 each), then the right one answers 429 with a
        `Retry-After` header;
  - [x] register, 4 wrong, the right one (200), then 5 wrong are all 401.

**Files:** `integration-test/limit_test.go`. **Verify:** G1, `go vet ./integration-test/...`; G3.
**Depends on:** 8. **Size:** S

### 10. Docs
- [x] `docs/configuration.md`: `TRUSTED_PROXY` (the one-proxy rule, Caddy's fixed address in
      compose) and the three `AUTH_LIMIT_*` settings.
- [x] `docs/api.md`: 429 on register and login, with `Retry-After`; `ResourceExhausted` on gRPC.
- [x] `docs/cqrs.md`: the Limit step in the pipeline.
- [x] `docs/architecture.md`: `pkg/clientip`, `internal/repo/limiter`, the client middleware.
- [x] `docs/request-flow.md`: the `limit` row in §4's pipeline table, the step lists, and
      `ClientAddress` in §1's middleware table.
- [x] `CLAUDE.md`:
  - [x] the pipeline steps paragraph;
  - [x] the error kinds in the handler contract;
  - [x] the buses built per binary;
  - [x] `pkg/` lists `clientip`.

**Files:** the five above. **Verify:** each page read through; links resolve. **Depends on:** 8, 9.
**Size:** M

### Checkpoint
- [ ] G3 green on the branch head, plus the forged-header check through Caddy.
- [ ] Spec success criteria 1–6 checked one by one.
- [ ] Push; G4 green on PR #52; the PR body gains the module; the PR is ready for review.
