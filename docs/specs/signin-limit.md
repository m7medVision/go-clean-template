# Spec: sign-in and sign-up attempts limited in Valkey

Module id: `signin-limit` · third of three in one PR (see [README.md](README.md#valkey)) · Depends
on `valkey` · Status: **spec approved 2026-11-08; plan in [signin-limit-plan.md](signin-limit-plan.md), tasks in
[signin-limit-todo.md](signin-limit-todo.md)**

Paths are checked against `feat/valkey` at `0b5c6f5` on 2026-11-08.

## Objective

Password sign-in and sign-up get a budget per client and per account, so neither a script
guessing passwords nor one creating accounts can run unchecked. The counters live in Valkey, so
every app instance shares them. When it is done:

- **per client:** each client address gets 20 sign-in attempts and 20 sign-up attempts per
  15 minutes, counted separately;
- **per account:** each account gets 5 wrong passwords per 15 minutes. A sign-in that succeeds
  clears that account's count;
- **when a budget is spent,** the attempt is refused before its handler runs:
  - REST answers `429 Too Many Requests` with `Retry-After` in seconds;
  - gRPC answers `ResourceExhausted`;
  - GraphQL answers `RATE_LIMITED` (no GraphQL operation signs in today; the case is there so
    every kind maps);
- **the client is the real one behind Caddy,** read from `X-Forwarded-For` only when the request
  comes from Caddy's address (`TRUSTED_PROXY`). From anyone else, the header is ignored;
- **the request logs** of both servers show that same resolved client;
- **if Valkey fails mid-run,** the limit lets the attempt through and logs a warning, so an outage
  of a counter store never stops every sign-in;
- the worker is unchanged: it has no limiter and sends neither message.

**Who it is for:**
- an operator, whose sign-in endpoint stops being a free password-guessing oracle;
- a developer reading the template, who sees a security rule as one pipeline step with its state
  behind a port, and no handler or controller knowing about it.

## Scope

| Message | Bus | Client budget | Account budget |
| --- | --- | --- | --- |
| `user/query.LogIn` | queries | yes, scope `sign-in` | yes: only `entity.ErrInvalidCredentials` counts; success clears it |
| `user/command.RegisterUser` | commands | yes, scope `sign-up` | no |

Passkey and SSO sign-in are not limited:
- a passkey can't be guessed;
- the SSO provider runs its own sign-in, with its own limits.

The limit covers every transport, because it is a step on the buses: REST and gRPC today, and
anything later.

## Tech stack

| Piece | Version | Why |
| --- | --- | --- |
| `github.com/valkey-io/valkey-go/valkeylimiter` | v1.0.79 (a module of its own) | the counters as one Valkey round trip each; built on our existing client through `ClientBuilder`, so its commands are traced like every other |

**Algorithm: fixed window** (the library's default). "20 per 15 minutes" means 20 per window that
starts with the first attempt. That is the easiest rule to read and to put in `Retry-After`. Its
known cost: a client can spend a budget at the end of one window and again at the start of the
next.

**Never `Close` it.** `RateLimiterClient.Close` closes the client it was built on, which here is
the app's shared client. The app closes that one, once.

## Configuration

| Env var | Default | Read by | |
| --- | --- | --- | --- |
| `TRUSTED_PROXY` | empty: no proxy | the app (HTTP and gRPC) | the one address `X-Forwarded-For` is read from: Caddy's. Empty means the connection's own address is always the client. An address that doesn't parse stops startup |
| `AUTH_LIMIT_CLIENT_ATTEMPTS` | `20` | the app | attempts per client per window, for each of sign-in and sign-up |
| `AUTH_LIMIT_ACCOUNT_FAILURES` | `5` | the app | wrong passwords per account per window |
| `AUTH_LIMIT_WINDOW` | `15m` | the app | the window both budgets use |

- **None can be switched off.** A value of zero or less stops startup (`valkeylimiter` refuses it),
  so there is no "off".
- **Where each value is set:**
  - Defaults are written only in `config/`.
  - `.env.example` carries the same values, empty `TRUSTED_PROXY` included.
  - Compose sets `TRUSTED_PROXY` to Caddy's fixed address.
  - The integration stack and CI set `AUTH_LIMIT_CLIENT_ATTEMPTS=1000`, because the suite signs up
    and in about 33 times from one address.

## Design

### The client address

```
browser 203.0.113.7 ──▶ Caddy 172.30.0.10 ──▶ app
                          X-Forwarded-For: 203.0.113.7  (Caddy writes it, discarding the client's)
```

- **Caddy stays stock.** With no `trusted_proxies`, its `reverse_proxy` discards any
  `X-Forwarded-For` a client sends and writes the address of the connection it received. That
  holds for `app.lvh.me` and for gRPC on `grpc.lvh.me`. The Caddyfile doesn't change.
- **Compose pins the address:**
  - `app_network` gets a fixed subnet (`172.30.0.0/24`) and Caddy a fixed address
    (`172.30.0.10`);
  - the app's environment sets `TRUSTED_PROXY: "172.30.0.10"`.

  Trusting the whole subnet would not be enough. The app's ports are also published on the host,
  and host connections can arrive from the network's gateway, inside that subnet. Only Caddy is
  trusted.
- **`pkg/clientip`** (new, no domain knowledge) holds the one rule:
  ```go
  func Resolve(peer netip.Addr, forwardedFor []string, proxy netip.Addr) netip.Addr {
  	peer = peer.Unmap().WithZone("")
  	if !proxy.IsValid() || peer != proxy {
  		return peer
  	}

  	hops := strings.Split(strings.Join(forwardedFor, ","), ",")

  	client, err := netip.ParseAddr(strings.TrimSpace(hops[len(hops)-1]))
  	if err != nil {
  		return peer
  	}

  	return client.Unmap().WithZone("")
  }
  ```
  The rightmost entry is the one our proxy wrote. Anything to its left came from the client.
  With one proxy, that rightmost entry is the whole rule.
- **Who calls it:**
  - **HTTP:** a new `httpmw.ClientAddress(proxy)`, the first middleware on the one mux, so REST
    and GraphQL get it. It reads `r.RemoteAddr` and `r.Header.Values("X-Forwarded-For")`.
  - **gRPC:** a new interceptor, first in the chain. It reads `peer.FromContext` and the
    `x-forwarded-for` metadata.
  - Both store the result with `entity.ContextWithClient(ctx, addr)`.
  - `httpmw.Logger`, `httpmw.Recovery` and the gRPC logger and recovery log
    `entity.ClientFromContext` instead of the raw connection address.
- **Not used:** chi's `middleware.RealIP` (it trusts headers from anyone), `X-Real-IP`, `Forwarded`
  and CDN headers. Only Caddy is supported (decided 2026-11-08).

### The step

`behavior.Limit(limiter repo.Limiter, l *slog.Logger) cqrs.Step`, right after `Stamp` on both
buses. A message opts in by implementing:

```go
type Limited interface {
	LimitScope() string
	LimitAccount() string
}
```

- `LogIn` returns `"sign-in"` and its email.
- `RegisterUser` returns `"sign-up"` and `""`: no account budget.

For a `Limited` message, the step does this:

1. Read the client with `entity.ClientFromContext`. A message sent without one gets no client
   budget; only `internal/app`'s own transports send these messages, and they always set one.
2. Count the attempt against the client's budget. If it's spent: `ErrTooManyAttempts`.
3. If there is an account, check (without counting) the account's budget. If it's spent:
   `ErrTooManyAttempts`.
4. Run the handler.
5. If it returned `ErrInvalidCredentials`, count one failure against the account. If it succeeded,
   clear the account's count.

**Steps 2 and 3 run before `Authorize` and `Validate`.** A malformed attempt still spends the
client's budget.

**Check, then count, is not atomic.** Under concurrent wrong passwords, an account can take a few
more than 5 before it locks. That's accepted: an exact count would need a script of our own instead
of the library.

**When Valkey fails or doesn't answer in 500 ms,** the step logs
`"sign-in limit unavailable"` at warning level, with the scope and the error, and lets the attempt
through. Sign-ins are rare enough that this can't flood the log.

### The port and its adapter

`repo.Limiter` goes in `internal/repo/contracts.go`, with a `go:generate` mock:

```go
type Limiter interface {
	TakeClient(ctx context.Context, scope string, client netip.Addr) (entity.Allowance, error)
	CheckAccount(ctx context.Context, scope, account string) (entity.Allowance, error)
	TakeAccountFailure(ctx context.Context, scope, account string) error
	ClearAccount(ctx context.Context, scope, account string) error
}
```

- `entity.Allowance` is `{Allowed bool; RetryAfter time.Duration}`.
- `entity.Limits` holds the two rules from config.
- **The adapter, `internal/repo/limiter`,** holds two `valkeylimiter` clients, one per rule, built
  on the app's client.
- **Keys:**
  - client: `limit:client:{<scope>:<addr>}`, where an IPv6 client is keyed by its `/64`, since one
    user usually holds the whole prefix;
  - account: `limit:account:{<scope>:<hash>}`, where `<hash>` is the hex SHA-256 of the
    lower-cased, trimmed email, so Valkey holds no email address.

### Errors and transports

- **`entity.KindRateLimited`,** a new kind. `exhaustive` fails the lint until all three mappers
  handle it.
- **`entity.ErrTooManyAttempts`,** message `"too many attempts"`.
  - The step returns it wrapped in an `*entity.RetryAfterError`, so `errors.Is` still finds it and
    the transports can read the wait.
  - It doesn't say which budget was spent, so it can't confirm whether an account exists.
- **REST:**
  - `apierr.Problem` answers `huma.ErrorWithHeaders(huma.Error429TooManyRequests(...),
    Retry-After)`, rounded up to whole seconds;
  - the `register` and `login` operations list 429 in their `Errors`;
  - then `make openapi`, `make web-api` and `make web-check`. The web forms already show the
    problem's message through `FormError`.
- **gRPC:** `rpcerr.Status` answers `codes.ResourceExhausted` with a `RetryInfo` detail.
- **GraphQL:** `gqlerr.Map` answers `RATE_LIMITED`.

### Wiring

- **Today `newBootstrap` builds the buses for both binaries,** but only the app has Valkey. So the
  bootstrap stops building them, and each binary calls `newBuses` itself:
  - the app: after `valkey.New`, with `behavior.Limit(limiter.New(client, limits), l)`;
  - the worker: with no limit step.

  `newBuses` takes the extra steps and puts them right after `Stamp`.
- **Config:**
  - `config.HTTP` gains `TrustedProxy` (`TRUSTED_PROXY`), parsed once in `internal/app` and handed
    to both servers' middleware;
  - a new `config.AuthLimit` section holds the three limit values.

## Project structure

| File | Change |
| --- | --- |
| `pkg/clientip/clientip.go`, `_test.go` | new: `Resolve`, table-driven tests (direct client, forged header, through the proxy, unparsable entry, IPv4-mapped, no proxy set) |
| `internal/entity/client.go` | new: `ContextWithClient`, `ClientFromContext` |
| `internal/entity/limit.go` | new: `Limits`, `Allowance`, `RetryAfterError`, `ErrTooManyAttempts`; `KindRateLimited` in `errors.go` |
| `internal/controller/httpmw/client.go`, `_test.go` | new: `ClientAddress`; `logger.go` and `recovery.go` log the resolved client |
| `internal/controller/grpc/middleware/client.go`, `_test.go` | new: the interceptor; `logger.go` and `recovery.go` log the resolved client |
| `internal/repo/contracts.go`, `repomock/contracts.go` | `Limiter`; `make mock` |
| `internal/repo/limiter/limiter.go` | new: the adapter over two `valkeylimiter` clients |
| `internal/usecase/behavior/behavior.go`, `_test.go` | `Limited`, `Limit` |
| `internal/usecase/user/query/log_in.go`, `command/register_user.go` | `LimitScope`, `LimitAccount` |
| `internal/controller/restapi/v1/apierr`, `grpc/v1/rpcerr`, `graphql/gqlerr` | the new kind |
| `internal/controller/restapi/v1/user/` (register, login ops) | 429 in `Errors`; `docs/openapi/v1.json`, `web/src/api/schema.gen.ts` regenerated |
| `internal/app/bootstrap.go`, `buses.go`, `app.go`, `worker.go`, `http.go`, `grpc.go` | the buses built per binary, the limit step, the client middleware and interceptor |
| `config/config.go`, `.env.example` | `TRUSTED_PROXY`, the `AuthLimit` section |
| `docker-compose.yml` | the pinned subnet, Caddy's address, `TRUSTED_PROXY` |
| `docker-compose-integration-test.yml`, `.github/workflows/ci.yml` | `AUTH_LIMIT_CLIENT_ATTEMPTS=1000` |
| `integration-test/limit_test.go` | new (see Testing) |
| `go.mod`, `go.sum` | `valkeylimiter` v1.0.79 |
| docs | `configuration.md` (the four settings, the Caddy rule), `api.md` (429 on register and login), `cqrs.md` (the Limit step), `architecture.md` (`pkg/clientip`, `internal/repo/limiter`), `CLAUDE.md` (the pipeline steps, the error kinds) |

## Code style

The repo's conventions, plus no comments unless one is important. Two are expected:
- on the check-then-count race;
- on why the step lets an attempt through when Valkey fails.

## Testing strategy

- **Unit (`make test`, no Docker):**
  - `pkg/clientip`: every row of the table in Design, as a table-driven test;
  - `httpmw.ClientAddress` and the gRPC interceptor: a request from the proxy with a header, one
    from elsewhere with a forged header, and no proxy configured;
  - `behavior.Limit`, against the generated `repomock.MockLimiter`:
    - a spent client budget refuses before the handler runs;
    - a spent account budget refuses;
    - `ErrInvalidCredentials` counts a failure;
    - success clears the account;
    - a message that isn't `Limited` passes untouched;
    - a limiter error lets the attempt through and logs;
  - the three mappers: 429 with `Retry-After` rounded up, `ResourceExhausted` with `RetryInfo`,
    `RATE_LIMITED`;
  - `internal/app`: the worker's buses have no limit step, the app's do, and both answer as before.
- **Integration (`integration-test/limit_test.go`):**
  - **The account budget:** register, then send 5 wrong passwords (401 each). The 6th attempt,
    even with the right password, answers 429 with a `Retry-After` header. After the window, it
    would succeed; the test doesn't wait for that, the unit tests cover it.
  - **A success clears the count:** register, 4 wrong passwords, the right one, then 5 more
    wrong ones are all 401.
  - **The adapter against the stack's Valkey:** a client budget of 2 runs out on the 3rd take, and
    an account cleared after failures counts from zero. It uses its own key scope, `_test`, so it
    can't touch the suite's.
  - The client budget isn't tested end to end, since the stack raises it to 1000; the step's unit
    tests cover it.
- **By hand, once:**
  - through Caddy (`app.lvh.me`), with a forged `X-Forwarded-For`: the app's log shows the real
    client;
  - 21 sign-ins: the 21st answers 429.

## Boundaries

- **Always:** G1 before each commit; `make openapi`, `make web-api` and `make web-check` after the
  REST errors change; the `exhaustive` cases on all three transports.
- **Ask first:**
  - limiting passkey or SSO sign-in;
  - failing closed when Valkey is down;
  - a CAPTCHA or delay instead of a refusal;
  - trusting any proxy besides Caddy.
- **Never:**
  - an on/off switch for the limit;
  - an email address stored in Valkey;
  - a 429 that says which budget was spent;
  - reading `X-Forwarded-For` from an address that isn't `TRUSTED_PROXY`;
  - calling `Close` on a `valkeylimiter` client.

## Success criteria

1. The 6th wrong password for one account within the window answers 429 with `Retry-After`, on
   REST, and `ResourceExhausted` on gRPC (integration test; gRPC by unit test).
2. A successful sign-in clears that account's count (integration test).
3. The 21st attempt from one client within the window answers 429, and a forged
   `X-Forwarded-For` from anywhere but `TRUSTED_PROXY` doesn't change the client (unit tests; by
   hand through Caddy).
4. With Valkey stopped, sign-in still works and the app logs one warning per attempt (unit test;
   by hand).
5. The request logs show the client behind Caddy (by hand).
6. The worker builds no limiter and starts without Valkey; G1, `make web-check` and the
   integration suite pass; CI is green; the docs in the table say what changed.

## Release

`feat(auth): limit sign-in and sign-up attempts per client and per account` and the commits
around it. The PR already releases a minor version through `feat(valkey)!`. This adds a 429 that
REST and gRPC clients may now see; it removes nothing, so it isn't breaking.

## Decided

- Only Caddy is supported as a proxy, as one address, `TRUSTED_PROXY`; no CDN (2026-11-08).
- Compose gives Caddy a fixed address and trusts only it (2026-11-08).
- The request logs show the resolved client (2026-11-08).

## Decided by default (approved with the spec, 2026-11-08)

1. **The numbers:** 20 attempts per client and 5 wrong passwords per account, per 15 minutes; the
   client budget counted separately for sign-in and sign-up.
2. **Only password sign-in and sign-up are limited;** passkey and SSO are not.
3. **A successful sign-in clears the account's failures.**
4. **Valkey failing lets attempts through** (fail open), with a warning per attempt. The other
   choice, fail closed, stops every password sign-in while Valkey is down.
5. **Fixed window,** the library's default, rather than GCRA.
6. **Emails are hashed** in the account keys.
7. **The buses are built per binary,** so only the app's carry the limit step.
8. **Locking an account out is possible:** anyone who knows an email can spend its 5 failures and
   block password sign-in for that account for up to 15 minutes. Passkey and SSO still work. A
   budget per client and account pair would avoid that, but it lets an attacker with many addresses
   guess without limit, so the account budget stays.
