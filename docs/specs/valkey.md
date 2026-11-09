# Spec: Valkey in the stack

Module id: `valkey` · first of three in one PR (see [README.md](README.md#valkey)) · Depends on
nothing · Status: **done: merged 2026-11-08 (#52), released in v0.4.0; plan in [valkey-plan.md](valkey-plan.md), tasks in [valkey-todo.md](valkey-todo.md)**

Paths and versions below are checked against master at `0aba6bd` (#51 merged) on 2026-11-08.

## Objective

Valkey becomes a required part of the stack the app runs on, ready for the two modules that use it:
`exchange-cache` (the daily rates) and `signin-limit` (the sign-in counters). This module adds no
behaviour a user sees. When it is done:

- the app refuses to start without `VALKEY_URL`, the way it refuses without `JWT_SECRET`;
- the app connects to Valkey and pings it at startup, fails with a clear error when it cannot, and
  closes the client when it stops;
- every Valkey command is traced at the client level, so no repository carries tracing code;
- every way of running the app has a Valkey: `make dev`, `make compose-up`, `make compose-up-all`,
  `make compose-up-integration-test`, and CI's integration job;
- the worker runs exactly as before, without `VALKEY_URL`.

**Who it is for:** a developer starting from the template, who gets Valkey with the rest of the stack
and never sets it up by hand; and an operator, who learns from the release notes and
`docs/configuration.md` that the app now needs one.

## Tech stack

| Piece | Version | Why |
| --- | --- | --- |
| Server | `valkey/valkey:9.1.3-alpine` | latest stable on 2026-11-08 (9.2 is a release candidate), pinned like `postgres:18.6-alpine` |
| Client | `github.com/valkey-io/valkey-go` v1.0.79 | the official pure-Go client; the build is `CGO_ENABLED=0`, which rules out Valkey GLIDE's Go client (a Rust core over cgo) |
| Tracing | `github.com/valkey-io/valkey-go/valkeyotel` v1.0.79 (a module of its own) | a span per command on the global tracer provider, as otelpgx does for SQL |

`valkeylimiter` arrives with `signin-limit`. `VALKEY_URL` uses the `valkey://` scheme;
`valkey.ParseURL` in v1.0.79 accepts `valkey`, `valkeys` (TLS), `redis`, `rediss` and `unix`.

The server runs with no persistence (`--save "" --appendonly no`): it holds only a cache and
counters, which may be lost on a restart.

## Configuration

| Env var | Default | Read by | |
| --- | --- | --- | --- |
| `VALKEY_URL` | required by the app | the app | `valkey://localhost:${VALKEY_PORT}/0` in `.env.example`, `valkey://valkey:6379/0` in compose. A password goes in the URL (`valkey://:secret@host:6379/0`), TLS is `valkeys://` |
| `VALKEY_PORT` | `6379` | compose, `.env.example` | the host port the compose `valkey` service publishes; `VALKEY_URL` follows it, as `PG_URL` follows `PG_PORT` |

`config.Valkey` has one field, `URL string \`env:"VALKEY_URL"\``, with no `required` tag: the
worker parses the same config and has no Valkey. `app.Run` checks it instead and returns
`ErrNoValkeyURL`, beside `ErrNoJWTSecret` (`internal/app/app.go:24`).

## Project structure

| File | Change |
| --- | --- |
| `pkg/valkey/valkey.go` | new: `New(ctx, url) (vk.Client, error)`: parse the URL, build the traced client, ping it |
| `pkg/valkey/valkey_test.go` | new: a bad URL and an unreachable address fail, without Docker |
| `config/config.go` | the `Valkey` section |
| `internal/app/app.go` | `ErrNoValkeyURL`; after the bootstrap, `valkey.New`, closed when `Run` returns |
| `internal/app/app_test.go` | `Run` without `VALKEY_URL` returns `ErrNoValkeyURL` before it opens Postgres |
| `docker-compose.yml` | the `valkey` service with a `valkey-cli ping` healthcheck; `app` waits for it to be healthy; `VALKEY_URL` in the backend environment |
| `docker-compose-integration-test.yml` | `VALKEY_URL` for the `integration-test` service |
| `Makefile` | `valkey` in `COMPOSE_SERVICES`, so `make compose-up` and `make dev` start it |
| `.env.example` | `VALKEY_PORT`, `VALKEY_URL` |
| `.github/workflows/ci.yml` | the integration job runs Valkey with `docker run` and waits for `valkey-cli ping` |
| `integration-test/valkey_test.go` | new: `valkey.New` against the stack's Valkey, and a round trip on a scratch key |
| `docs/configuration.md` | a Valkey section with both variables |
| `docs/development.md` | Valkey in the service tables |
| `docs/architecture.md` | `pkg/valkey` in the Packages table |
| `README.md` | "Postgres and nothing else" becomes Postgres for everything durable and Valkey for the cache and counters |
| `CLAUDE.md` | the app needs Valkey as it needs the worker; `pkg/valkey` in the dependency notes |

The `deps` struct gets the client in `exchange-cache`, its first user: a field nothing reads would
fail `unused`.

## Code style

The repo's conventions, plus the user's rule for new code: **no comments unless one is important**
(a reason the code cannot say: a security rule, an invariant, a workaround). Errors wrap with the
layer path; the library's package is imported as `vk`, since ours is `valkey` too.

```go
package valkey

import (
	"context"
	"fmt"

	vk "github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeyotel"
)

func New(ctx context.Context, url string) (vk.Client, error) {
	opt, err := vk.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("valkey - New - vk.ParseURL: %w", err)
	}

	client, err := valkeyotel.NewClient(opt)
	if err != nil {
		return nil, fmt.Errorf("valkey - New - valkeyotel.NewClient: %w", err)
	}

	if err = client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()

		return nil, fmt.Errorf("valkey - New - ping: %w", err)
	}

	return client, nil
}
```

## Testing strategy

- **Unit (`make test`, no Docker):** `pkg/valkey` fails on a URL it cannot parse and on an address
  nothing listens on; `internal/app` returns `ErrNoValkeyURL` with an empty URL. Table-driven where
  there are cases, `t.Parallel()` everywhere.
- **Integration (`make compose-up-integration-test`, and CI):** `integration-test/valkey_test.go`
  connects with the `VALKEY_URL` it is given, then sets, reads and deletes a key under `_test:`,
  which no module uses. The rest of the suite passing proves the app started, since it refuses to
  start without Valkey.
- **By hand, once:** the startup `PING` span shows in Jaeger.

## Boundaries

- **Always:** `make format`, `make linter-golangci` and `make test` before each commit; pinned
  versions; the docs in the table above in the same commit series.
- **Ask first:** persistence for Valkey; a third use of Valkey beyond `exchange-cache` and
  `signin-limit`; Valkey in the worker; changes to CI outside the integration job.
- **Never:** a `required` tag on `VALKEY_URL` (it stops the worker); Valkey imported by `entity`,
  `usecase` or a controller; anything durable stored in Valkey; a Valkey password committed outside
  `.env.example`'s development defaults.

## Success criteria

1. `app.Run` with an empty `VALKEY_URL` returns `ErrNoValkeyURL`, before it opens Postgres.
2. With `VALKEY_URL` pointing at nothing, the app exits at startup with an error naming
   `valkey.New`.
3. `make dev`, `make compose-up-all` and `make compose-up-integration-test` start Valkey, and the
   app reaches it.
4. CI's integration job is green with Valkey in it.
5. The worker starts with no `VALKEY_URL` set.
6. `make format`, `make linter-golangci` and `make test` pass; the docs above say what changed.

## Decided

- Valkey is required by the app and unknown to the worker (2026-11-08).
- One PR for `valkey`, `exchange-cache` and `signin-limit` (2026-11-08).
- The design in the conversation of 2026-11-08 is approved: the client in `pkg/valkey`, chosen in
  `internal/app`; the cache inside the exchange adapter behind a port the adapter owns; the limit
  as a `behavior.Limit` step behind `repo.Limiter`.
- No comments in new code unless important (2026-11-08).
- The release marker is `feat(valkey)!:` with a `BREAKING CHANGE: the app needs VALKEY_URL`
  footer: a required `VALKEY_URL` breaks every deployment that does not set one (2026-11-08).
- `VALKEY_PORT` stays, mirroring `PG_PORT`, for a host that already runs something on 6379
  (2026-11-08).
