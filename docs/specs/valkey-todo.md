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
- [ ] `docker-compose.yml`: a `valkey` service (`valkey/valkey:9.1.3-alpine`,
      `--save "" --appendonly no`, host port `${VALKEY_PORT:-6379}`, a `valkey-cli ping`
      healthcheck, alias `valkey.lvh.me`); `VALKEY_URL: "valkey://valkey:6379/0"` in the backend
      environment; `app` waits for `valkey` to be healthy.
- [ ] `docker-compose-integration-test.yml`: `VALKEY_URL` for the `integration-test` service.
- [ ] `Makefile`: `valkey` in `COMPOSE_SERVICES`, so `make compose-up` and `make dev` start it.
- [ ] `.env.example`: `VALKEY_PORT=6379`, `VALKEY_URL=valkey://localhost:${VALKEY_PORT}/0`.
- [ ] `ci.yml` integration job: `docker run` Valkey on 6379 and wait for `valkey-cli ping`.

**Files:** `docker-compose.yml`, `docker-compose-integration-test.yml`, `Makefile`, `.env.example`,
`.github/workflows/ci.yml`.
**Verify:** G1; G2: `docker compose up -d valkey` is healthy and answers `PONG`.
**Depends on:** none. **Size:** M

### 3. The app requires Valkey
- [ ] `config.Valkey{URL}` read from `VALKEY_URL`, no `required` tag.
- [ ] `app.Run`: `ErrNoValkeyURL` before the bootstrap, beside `ErrNoJWTSecret`; `valkey.New` after
      it; the client closed when `Run` returns.
- [ ] `internal/app/app_test.go`: an empty `VALKEY_URL` is `ErrNoValkeyURL`, before Postgres opens.
- [ ] Commit body carries `BREAKING CHANGE: the app needs VALKEY_URL`.

**Files:** `config/config.go`, `internal/app/app.go`, `internal/app/app_test.go`.
**Verify:** G1; G2: `make run` starts with Valkey up, and with Valkey stopped exits naming
`valkey.New`; the worker starts with `VALKEY_URL` unset.
**Depends on:** 1, 2. **Size:** M

### 4. The integration suite reaches Valkey
- [ ] `integration-test/valkey_test.go`: `valkey.New` with the `VALKEY_URL` the stack gives, then
      set, read and delete a key under `_test:`.

**Files:** `integration-test/valkey_test.go`.
**Verify:** G1 (`go vet ./integration-test/...`); G3. **Depends on:** 2, 3. **Size:** S

### 5. Docs
- [ ] `docs/configuration.md`: a Valkey section with `VALKEY_URL` and `VALKEY_PORT`.
- [ ] `docs/development.md`: Valkey in the service tables.
- [ ] `docs/architecture.md`: `pkg/valkey` in the Packages table.
- [ ] `README.md`: "Postgres and nothing else" becomes Postgres for everything durable, Valkey for the
      cache and counters.
- [ ] `CLAUDE.md`: the app needs Valkey; `pkg/valkey` in the dependency notes.

**Files:** the five above. **Verify:** each page read through; links resolve. **Depends on:** 3.
**Size:** M

### Checkpoint
- [ ] G3 green on the branch head.
- [ ] Spec success criteria 1–6 checked one by one.
- [ ] Then the `exchange-cache` spec.
