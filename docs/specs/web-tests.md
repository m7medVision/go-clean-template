# Spec: web screen tests

Module id: `web-tests` · fourth of six (see [web.md](web.md)) · Depends on `web-boundaries` ·
Status: **done: merged 2026-11-08 (#53), released in v0.5.0**

Facts below were checked in `web/` on 2026-11-08 (Vitest 5.0.5 through Vite+, `vp test`).

## Objective

The sign-in, tasks and exchange screens are rendered in tests, against an HTTP layer typed from the
API's own contract, so a change that breaks what the user does (a field renamed, a status code
handled wrong) fails `make web-check` instead of showing up in the browser. Done means:

- a test setup that renders a route inside the real router and a fresh `QueryClient`;
- HTTP handlers for REST and GraphQL whose request and response shapes are checked by the compiler
  against `web/src/api/schema.gen.ts` and `web/src/graphql/`;
- tests of these flows, one file each: sign in (success, wrong password, rate-limited), tasks (list,
  create, change status, delete), exchange (convert, history);
- all of it running in `make web-check`.

## What exists today

- 20 test files, 124 tests, all of pure logic (schemas, token, SSO and passkey helpers) plus one
  `renderToStaticMarkup` test. No test renders a screen or touches the network layer.
- No DOM environment: `jsdom` and `happy-dom` are not installed. `@testing-library/dom` and
  `user-event` are in `node_modules` only as dependencies of Vitest's browser mode.
- No `msw`. No test imports the router.

## Decisions

1. **MSW intercepts `fetch`**, so the real `openapi-fetch` client and the real `request()` run, and
   a handler returns what the Go API returns. Mocking `$api` or `request` would test the mocks.
2. **Handlers are typed from the generated contract**: a response body is
   `components["schemas"][…]` and a GraphQL result is the operation's generated result type, so an
   API change breaks the handler at compile time. A helper wraps `http.get/post` with the path
   typed from `paths`.
3. **`happy-dom`** as the DOM (lighter than `jsdom`; the screens use no layout APIs). If a test
   needs `jsdom`, the plan switches; the choice is per-file via a docblock, so it is not a rewrite.
4. **Component-level, not page-level, where a route is heavy**: a route's component is rendered
   with a memory-history router and its loader run, not the whole app shell, except for the sign-in
   flow, which also checks the redirect.
5. **No snapshots.** Tests assert on roles and text a user sees.

## Dependencies (each needs your approval)

| Package | Why |
| --- | --- |
| `happy-dom` | the DOM environment |
| `@testing-library/react`, `@testing-library/user-event` | render and interact the way a user does; `user-event` is already in the tree |
| `msw` | fetch-level API mocking |

All as `devDependencies`, versions pinned in the plan from the lockfile's resolution.

## Project structure

| File | Change |
| --- | --- |
| `web/src/testing/setup.ts` | MSW server start/reset/stop; clears `sessionStorage` and the token between tests |
| `web/src/testing/handlers.ts` | typed REST and GraphQL handler helpers |
| `web/src/testing/render.tsx` | `renderRoute(path)`: memory-history router, fresh `QueryClient` (no retries), the app's providers |
| `web/vite.config.ts` | `test.environment`, `test.setupFiles` |
| `web/src/<domain>/*.test.tsx` | the flows above, beside the code |
| `docs/web.md` | a "Testing" section: the helpers and how to add a flow |

`testing/` is shared code for the layer rules and may be imported by tests of any layer; it imports
no domain. Domain tests that need domain fixtures define them beside themselves.

## Commands

```
make web-check     # now includes the screen tests
make web-format
```

## Testing the tests

- Each flow gets one deliberate break to prove it fails: rename a field in a handler's body, and
  change a status in `tasks/status.ts`; the matching test must go red. Recorded in the PR.
- The suite's added time stays under 10 seconds on this machine (the whole suite is 0.6 s today).

## Boundaries

- Always: assert on what a user sees; reset the handlers and the cache between tests; keep `act`
  warnings at zero.
- Ask first: each dependency above; a real timer or a sleep in a test.
- Never: mock `fetch` by hand, `request`, `$api` or the router; skip a failing test.

## Success criteria

1. `renderRoute("/tasks")` shows the handler's tasks in the data table.
2. The six flows listed pass, and each fails on its deliberate break.
3. A handler that returns a body not matching the OpenAPI schema fails type checking.
4. `make web-check` passes, with the added time under 10 s.

## Open questions

1. Approve the three dependency groups? `happy-dom` over `jsdom`?
2. Is covering only sign-in, tasks and exchange enough for the first PR (passkeys and SSO stay
   pure-logic tests)? I assume yes.
