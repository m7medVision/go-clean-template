# Capability map: web/ structure and quality

Status: **capability map approved 2026-11-08. All six module specs are written; plans follow each approval.**
Idea source: [bulletproof-react](https://github.com/alan2207/bulletproof-react) (project structure,
testing, error handling, security), compared with `web/` on 2026-11-08.

## Objective

`web/` keeps its generated types, data-table and form conventions (already stricter than
bulletproof-react's) and gains what it lacks: import rules a tool enforces, tests that exercise
screens, contained failures, one place for role checks, and production error reports. Nothing here
changes what a user sees when everything works.

## What `web/` has today (checked 2026-11-08)

- Domain folders `tasks/`, `exchange/`, `passkey/`, `sso/`, `user/`, shared `api/`, `auth/`,
  `components/`, `hooks/`, `lib/`; routes in `routes/`.
- Nothing enforces imports. Today `passkey/` imports `#/user/api` (a domain importing a domain).
- One `errorComponent` (`routes/_authed/admin/tasks.tsx`); no error boundary anywhere else.
- `auth/roles.ts` reads roles from the token for the admin nav link only; no shared gate component.
- Tests are `*.test.ts` of pure logic, and one `.tsx`. No screen is rendered in a test; no e2e.
- No frontend error reporting. GlitchTip is the chosen tracker (Sentry SDK only), not yet in the stack.

## Capability map

| Module id | Responsibility | Depends on |
| --- | --- | --- |
| `web-boundaries` | The import rules (shared → domains → routes; a domain never imports a domain) as a lint rule or test that fails the build; the few existing violations fixed; the folder map in `docs/web.md` | — |
| `web-errors` | One error boundary per route and per dialog, and one place that handles API failures; a failure in one screen leaves the shell usable | — |
| `web-authz` | One gate component and policy helpers for role and ownership checks in the UI (server stays the enforcement) | `web-boundaries` |
| `web-tests` | Testing Library tests that render the sign-in, tasks and exchange screens against HTTP handlers typed from `docs/openapi/v1.json`; a `make web-check` that runs them | `web-boundaries` |
| `web-e2e` | A small Playwright suite (sign in → create task → see it listed) run against the real stack, locally and in CI | `web-tests` |
| `web-error-tracking` | Frontend errors reported to GlitchTip through the Sentry SDK, source maps uploaded by the build; needs a GlitchTip service and DSN setting in the stack | `web-errors` |

Build order: `web-boundaries` → `web-errors`, `web-authz`, `web-tests` → `web-e2e`,
`web-error-tracking`. `web-boundaries` goes first because every later module adds files, and the
rule should already be checking them.

## Not in scope

- Moving the JWT out of memory/`sessionStorage` into an `HttpOnly` cookie: a backend change (refresh
  endpoint, cookie, CSRF) that deserves its own spec.
- Reshaping domain folders into `api/ components/ hooks/` subfolders: revisit only if a domain
  passes about eight files.
- Anything under Go packages.

## Assumptions (correct me now)

1. Each module ships as its own PR, in the order above, as with the Valkey and Mongo specs.
2. The boundary rule is a lint rule if Oxlint in Vite+ can express it; otherwise a Vitest test that
   scans imports. Decided in the `web-boundaries` spec after I check what Oxlint supports.
3. `web-error-tracking` waits for the GlitchTip rollout in the backend; if that is not scheduled,
   the module is deferred, not built against a stub.
4. No new runtime dependency lands without asking, per `CLAUDE.md` practice: Testing Library, MSW,
   Playwright and the Sentry SDK each need your approval in their module's spec.

## Open questions

1. Approve the six modules, their boundaries and the order? Cut or merge any (for example, fold
   `web-authz` into `web-errors`)?
2. Is `web-error-tracking` in this series, or after the backend GlitchTip work?
3. Should `web-e2e` be required in CI, or a non-blocking job at first?
