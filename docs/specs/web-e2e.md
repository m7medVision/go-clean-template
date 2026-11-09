# Spec: web end-to-end tests

Module id: `web-e2e` · fifth of six (see [web.md](web.md)) · Depends on `web-tests` ·
Status: **done: merged 2026-11-08 (#53), released in v0.5.0; see web-e2e-todo.md for the one open check**

Facts below were checked in the repo on 2026-11-08.

## Objective

One short Playwright suite drives a real browser through the real stack (app, worker, Postgres,
Valkey) so the contract between the Go service and the web client is exercised without mocks. It is
small on purpose: it proves the path, the screen tests (`web-tests`) cover the cases. Done means:

- `make web-e2e` runs: sign up → sign in → create a task → see it in the list → change its status
  → sign out;
- the suite runs in CI in its own job;
- it leaves the database as it found it, apart from its own throwaway user.

## What exists today

- No browser test, no `playwright` dependency.
- The Go app does not serve the client. In compose, Caddy serves a built `web/dist` and proxies the
  API; in development, the Vite dev server proxies to the app (`make dev`, `make web-dev`).
- CI's integration job runs the app and worker **on the runner**, not in compose (Postgres by
  `setup-postgres`, `.env.example` sourced, other services by `docker run`). Valkey is needed there
  now. `ci.yml` has a `web` job (`make web-check`, `make web-build`) and an `integration-tests` job.
- The Go integration suite lives in `integration-test/` and runs inside the compose network.

## Decisions

1. **Run against the Vite dev server proxying to the app**, not the Caddy stack: no WAF, cache or
   TLS in the way, and the same wiring `make web-dev` uses. A production build under Caddy is not
   tested here.
2. **A new CI job, not the integration job.** It starts the same dependencies as the integration job
   (Postgres, Valkey, the app, the worker) but not Mailpit, Keycloak or the exchange stub, which the
   path does not use; the plan checks whether `.env.example` needs either to start the app.
3. **Chromium only.**
4. **Test data is a fresh user per run** (random email), so a re-run never collides and nothing is
   cleaned up. Sign-up is open (`POST /v1/auth/register`), so no seeding is needed.
5. **Not blocking at first** (your open question 3 in [web.md](web.md)): the job is added to CI but
   left out of `ci-ok`'s `needs` for the first two weeks, then added. Say if you want it blocking
   from the start.

## Dependencies (need your approval)

| Package | Why |
| --- | --- |
| `@playwright/test` (dev) | the runner and API; browsers installed by `playwright install chromium` in CI |

## Project structure

| File | Change |
| --- | --- |
| `web/e2e/playwright.config.ts` | `baseURL` from `WEB_PORT`, one project (chromium), `webServer` running `make web-dev`-equivalent |
| `web/e2e/tasks.spec.ts` | the path above |
| `Makefile` | `web-e2e` target: needs the stack up (documented), runs `vp -C web exec playwright test` |
| `.github/workflows/ci.yml` | the `web-e2e` job (new services follow the "CI runs on the runner" pattern) |
| `docs/web.md`, `docs/development.md` | how to run it locally and what it needs |

`web/e2e/` is outside `src/`, so the layer rules do not apply to it, and `vp check` must lint and
type-check it.

## Commands

```
make dev           # or: make compose-up, make run, make run-worker, in other terminals
make web-e2e       # playwright test against WEB_PORT
```

`make web-e2e` fails with a clear message if the app is not answering on `HTTP_PORT`.

## Testing

- The suite is the test. Proven by breaking: change a label on the tasks screen and the spec must
  fail; the Playwright trace is kept on failure as a CI artifact.
- Run it 3 times in a row locally: the same result each time (no leftover state).

## Boundaries

- Always: unique user per run; wait on visible state, never on a fixed delay; keep the suite under
  about 30 seconds.
- Ask first: the Playwright dependency; making the job blocking; adding a second spec.
- Never: reuse a named account; talk to the database from the test; run it against a deployed
  environment.

## Success criteria

1. `make web-e2e` passes locally against `make dev`, three times in a row.
2. The CI job passes on the PR and uploads a trace on failure (checked by one deliberate failure).
3. The job's added CI time is under 4 minutes.
4. `make web-check` is unaffected.

## Open questions

1. Playwright and its browser download in CI: approved?
2. Blocking from the first merge, or after two weeks (decision 5)?
3. Should the suite also cover the passkey and SSO sign-in buttons? Both need the browser's
   WebAuthn or a Keycloak: I assume no, they stay in the Go integration suite.
