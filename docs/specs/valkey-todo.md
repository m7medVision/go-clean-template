# Tasks: Valkey in the stack

Plan: [valkey-plan.md](valkey-plan.md) · Spec: [valkey.md](valkey.md)

Gates (G1–G4) are defined in the plan. Every task: one Conventional Commit, the box ticked here, no
comments in new code unless important. Sizes: S = 1–2 files, M = 3–5.

---

### 1. A traced client in `pkg/valkey`
- [x] `valkey.New(ctx, url)` parses the URL, builds the client with `valkeyotel.NewClient`, pings it,
      and closes it when the ping fails.
- [x] Unit tests, table-driven, no Docker: a URL it cannot parse fails; a closed port on 127.0.0.1
      fails.
- [x] `valkey-go` and `valkeyotel` v1.0.79 in `go.mod`; `make deps` diff read.

**Files:** `pkg/valkey/valkey.go`, `pkg/valkey/valkey_test.go`, `go.mod`, `go.sum`.
**Verify:** G1. **Depends on:** none. **Size:** M

### 2. Valkey in every stack
- [x] `docker-compose.yml`: a `valkey` service (`valkey/valkey:9.1.3-alpine`,
      `--save "" --appendonly no`, host port `${VALKEY_PORT:-6379}`, a `valkey-cli ping`
      healthcheck, alias `valkey.lvh.me`); `VALKEY_URL: "valkey://valkey:6379/0"` in the backend
      environment; `app` waits for `valkey` to be healthy.
- [x] `docker-compose-integration-test.yml`: `VALKEY_URL` for the `integration-test` service.
- [x] `Makefile`: `valkey` in `COMPOSE_SERVICES`, so `make compose-up` and `make dev` start it.
- [x] `.env.example`: `VALKEY_PORT=6379`, `VALKEY_URL=valkey://localhost:${VALKEY_PORT}/0`.
- [x] `ci.yml` integration job: `docker run` Valkey on 6379 and wait for `valkey-cli ping`.

**Files:** `docker-compose.yml`, `docker-compose-integration-test.yml`, `Makefile`, `.env.example`,
`.github/workflows/ci.yml`.
**Verify:** G1; G2: `docker compose up -d valkey` is healthy and answers `PONG`.
**Depends on:** none. **Size:** M

### 3. The app requires Valkey
- [x] `config.Valkey{URL}` read from `VALKEY_URL`, no `required` tag.
- [x] `app.Run`: `ErrNoValkeyURL` before the bootstrap, beside `ErrNoJWTSecret`; `valkey.New` after
      it; the client closed when `Run` returns.
- [x] `internal/app/app_test.go`: an empty `VALKEY_URL` is `ErrNoValkeyURL`, before Postgres opens.
- [x] Commit body carries `BREAKING CHANGE: the app needs VALKEY_URL`.

**Files:** `config/config.go`, `internal/app/app.go`, `internal/app/app_test.go`.
**Verify:** G1; G2: `make run` starts with Valkey up, and with Valkey stopped exits naming
`valkey.New`; the worker starts with `VALKEY_URL` unset.
**Depends on:** 1, 2. **Size:** M

### 4. The integration suite reaches Valkey
- [x] `integration-test/valkey_test.go`: `valkey.New` with the `VALKEY_URL` the stack gives, then
      set, read and delete a key under `_test:`.

**Files:** `integration-test/valkey_test.go`.
**Verify:** G1 (`go vet ./integration-test/...`); G3. **Depends on:** 2, 3. **Size:** S

### 5. Docs
- [x] `docs/configuration.md`: a Valkey section with `VALKEY_URL` and `VALKEY_PORT`.
- [x] `docs/development.md`: Valkey in the service tables.
- [x] `docs/architecture.md`: `pkg/valkey` in the Packages table.
- [x] `README.md`: "Postgres and nothing else" becomes Postgres for everything durable, Valkey for the
      cache and counters.
- [x] `CLAUDE.md`: the app needs Valkey; `pkg/valkey` in the dependency notes.

**Files:** the five above. **Verify:** each page read through; links resolve. **Depends on:** 3.
**Size:** M

### Checkpoint
- [x] G3 green on the branch head: 39 of 39 integration tests in an isolated compose project at
      `4517b7d`; the one commit after it changes only docs.
- [x] Spec success criteria checked one by one:
  1. `TestTheAppRefusesToStartWithoutAJWTSecretOrAValkeyURL` passes.
  2. With Valkey stopped, `make run` exits with `app - Run - valkey.New: valkey - New -
     valkeyotel.NewClient: dial tcp [::1]:6379: connect: connection refused`.
  3. The integration stack (compose with the integration overrides) and `make run` against
     `db` and `valkey` reach Valkey; `make dev` and `make compose-up-all` start the same
     services from the same definitions and were not launched themselves.
  4. **Open:** CI runs at G4, after the push.
  5. The worker starts and runs with `VALKEY_URL` empty.
  6. G1 passed at every commit; the docs say what changed.
- [ ] Then the `exchange-cache` spec.
