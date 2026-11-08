# Implementation plan: web error handling

Spec: [web-errors.md](web-errors.md) · Tasks: [web-errors-todo.md](web-errors-todo.md) · Map: [web.md](web.md)

## Overview

The second of six `web/` modules, one PR (on top of `web-boundaries`), three tasks, one commit each:
- first the two components, tested, that nothing calls yet;
- then the router defaults that turn them on, with the retry proven by hand;
- then the docs.

## Order, and why

```
1. the components      RouteError (message, Try again) and RouteNotFound, with unit tests
2. turn them on        defaultErrorComponent and defaultNotFoundComponent in router.tsx; retry proven
3. docs                docs/web.md "Errors" paragraph; specs marked done after merge
```

- **Task 1 adds code nothing calls**, so it cannot change behaviour; it is tested on its own.
- **Task 2 is the one `feat` commit.** It is also where the open technical question is settled:
  what "Try again" must reset. `router.invalidate()` re-runs the loaders; a query that failed inside
  `useSuspenseQuery` also keeps its error in the cache. The first attempt is `router.invalidate()`
  plus `useQueryErrorResetBoundary().reset()` (the pairing TanStack documents); if the failed query
  is still served from the cache, `queryClient.resetQueries` on the failed keys instead. The
  component's props are the same either way.

## Commits

| Task | Subject |
| --- | --- |
| 1 | `refactor(web): a shared route error view and not-found page` |
| 2 | `feat(web): show route errors in the page with a way to retry` |
| 3 | `docs(web): how route errors are shown` |

Task 2 is a `feat`, so merging releases a minor version.

## Gates

- **G1** `make web-format` and `make web-check` pass.
- **G2** `make web-build` passes.
- **G3** the manual run in the spec's success criteria 1 to 4, recorded in the PR description.

## Risks

| Risk | Mitigation |
| --- | --- |
| Try again re-throws the cached error | task 2 tries `router.invalidate()` with the query reset first, then `resetQueries`; G3 criterion 1 is the proof |
| An error view hides the shell because it renders at the root | the `_authed` layout's children error inside its `<Outlet />`; criterion 1 checks the sidebar stays |
| `RouteNotFound` renders without the router's `Link` context | it renders inside the router, as a default component; the unit test wraps nothing and so uses a plain `<a>` only if `Link` cannot render statically (decided in task 1) |
| Retrying an operation that is not safe to repeat | only loaders and queries are retried; mutations never reach this view |
