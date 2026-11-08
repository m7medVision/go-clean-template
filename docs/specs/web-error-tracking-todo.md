# Tasks: web error tracking

Plan: [web-error-tracking-plan.md](web-error-tracking-plan.md) · Spec: [web-error-tracking.md](web-error-tracking.md)

- [x] `@sentry/react`; `lib/error-tracking.ts` (`initErrorTracking`, `reportError`, `scrubEvent`); `api/report.ts` (`shouldReport`); `tokenSubject`; wired in `main.tsx`, the query and mutation caches, and `RouteError`.
- [x] Unit tests: `shouldReport` (8 reported, 7 not), `scrubEvent` (request, breadcrumbs, console, user, rejected cause).
- [x] Browser check 2026-11-08 against a local envelope-capture server, not GlitchTip: a 500 sent one event with the user id, the request URL without its query string and no `hunter2`/`Bearer`; a 401 sent none; with no DSN the SDK made no request. The first run found a console breadcrumb carrying the URL's search params; `scrubEvent` now drops console breadcrumbs.
- [x] `VITE_SENTRY_DSN` in `.env.example` and `docs/configuration.md`; "Error tracking" in `docs/web.md`.
- [x] Source maps: `@sentry/vite-plugin` in `vite.config.ts`, active only when `SENTRY_AUTH_TOKEN` is set; hidden maps, debug-ID upload, deleted from `dist/` (0 `.map` files after the build).
- [x] Verified 2026-11-08 against a real GlitchTip (`glitchtip/glitchtip:latest`, run locally in Docker with Postgres and Valkey; not the project's future deployment) and the production build: a 500 gave one event with the frame `src/api/graphql.ts` / `request` / line 43 and its source line, `user: {id: user-123}`, `release: scratch-6`, no query string and no headers; an unhandled rejection was reported with the same release. Found on the way: a first query failing while the SDK was still loading was dropped (fixed: reports wait for a pending init, with a test); a dashed-UUID DSN key silently disables the SDK (documented).
- [x] A render crash: `components/render-crash.test.tsx` makes `/tasks` throw while rendering (the API answers without the list); the error view appears with the sidebar still present and `reportError` receives the `TypeError`. Fails when the report call is removed.
- [ ] Source-map upload against a real deployment: there is none yet (the backend GlitchTip module does not exist), so this stays open until one does.
