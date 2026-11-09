# Spec: web error handling

Module id: `web-errors` · second of six (see [web.md](web.md)) · Depends on nothing ·
Status: **done: merged 2026-11-08 (#53), released in v0.5.0**

Facts below were checked in `web/src` on 2026-11-08.

## Objective

When a screen's data fails to load or a screen crashes while rendering, the user sees a short message
and a way to try again, inside the page, with the sidebar and breadcrumb still usable. An unknown URL
shows a "not found" page. Nothing changes when everything works. Done means:

- every route has an error view, not only `/admin/tasks`;
- the error view offers "Try again", which reloads that route's data;
- an unknown URL shows a not-found page with a link back to `/tasks`;
- the existing sign-out on a rejected token is untouched.

## What exists today

- 401 handling is already central and covers both transports: `rejectSession()` in `api/client.ts`
  clears the token and runs the `onUnauthorized` handler set in `router.tsx`; `api/graphql.ts` calls
  it for `UNAUTHENTICATED`. This module does not touch it.
- `problemMessage` (`api/problem.ts`) turns a Huma problem or a GraphQL error into one line.
- Mutation failures are values, not exceptions: `<FormError error={mutation.error} />` shows them in
  forms and dialogs.
- Exactly one `errorComponent`: `AllTasksError` on `/admin/tasks`. Every other route falls back to
  the router's built-in error box. There is no `defaultNotFoundComponent`.
- The sidebar's profile query is `useQuery` (not suspense), so it never throws into rendering.
- No toast library and no error-boundary library are installed. `@tanstack/react-router` already
  exports `CatchBoundary` and `ErrorComponent`.

## Decisions (where I differ from the idea list)

1. **Router defaults, not a boundary per route.** A route's error view replaces only that route's
   `<Outlet />`, so with `defaultErrorComponent` set on the router every route is contained and the
   `_authed` shell survives. One component, one line in `createRouter`.
2. **No boundary per dialog.** A dialog's server failure is shown by `FormError`, never thrown, so a
   boundary would guard against nothing. Add one when a dialog first renders data that can throw.
3. **No toasts and no new dependency.** Errors appear in the page where the data would have been.
4. **`AllTasksError` stays.** Its heading and message are specific to that page; the default covers
   the rest.

## Behaviour

| Case | Result |
| --- | --- |
| A loader or `useSuspenseQuery` rejects (4xx/5xx, network) | in the route area: heading "Something went wrong", `problemMessage(error)` in an element with `role="alert"`, a "Try again" button |
| A component throws while rendering | the same view |
| "Try again" | clears the failed router match and the failed queries, then reloads the route (`router.invalidate()` plus `QueryErrorResetBoundary`'s reset, or `queryClient.resetQueries` on the failed keys; the plan picks after trying) |
| An error from the `_authed` layout itself | root-level view: same component, full page, no shell |
| Unknown URL | "Page not found" as a full page (root-level, no sidebar), link to `/tasks` |
| 401 / `UNAUTHENTICATED` | unchanged: sign-out and redirect to `/login` |

The message is the server's `problemMessage` text or its fallback, never a stack trace or an error
class name.

## Project structure

| File | Change |
| --- | --- |
| `web/src/components/route-error.tsx` | new: `RouteError` (error view with Try again) and `RouteNotFound` |
| `web/src/router.tsx` | `defaultErrorComponent: RouteError`, `defaultNotFoundComponent: RouteNotFound` |
| `web/src/components/route-error.test.tsx` | new: renders both, as `admin-error.test.tsx` does |
| `docs/web.md` | an "Errors" paragraph under "Talking to the API" |

Both components are shared code (`components/`), so they import only `api/problem` and the router's
own types, which the layer rules already allow.

## Commands

```
make web-format
make web-check     # format check, lint, type check, tests
make web-build
```

## Testing

- Unit, as today (`renderToStaticMarkup` from `react-dom/server`, no new dependency): `RouteError`
  shows the problem's `detail` with `role="alert"`, falls back to the generic line for an error
  without a body, and renders a button; `RouteNotFound` renders a link to `/tasks`.
- The retry behaviour needs a rendered router, which the `web-tests` module will provide; until then
  it is checked by hand once and the steps are recorded in the PR: stop the Go app, open `/tasks`
  (error view appears, sidebar stays), start the app, press Try again (the list loads).

## Boundaries

- Always: show only `problemMessage` text; keep `role="alert"` on the message; run `make web-format`
  and `make web-check`.
- Ask first: adding a dependency (a toast or error-boundary library); changing `rejectSession` or the
  401 flow; wrapping `RouterProvider` in anything.
- Never: show `error.stack` or `error.message` of a non-API error; swallow an error without showing
  it; touch Go packages.

## Success criteria

1. Opening `/tasks`, `/exchange` and `/profile` with the API down shows the error view in the route
   area with the sidebar still present, and Try again recovers once the API is back.
2. `/no-such-page` shows the not-found page with a link to `/tasks`, signed in or not (full page, no sidebar).
3. `/admin/tasks` as a non-admin still shows `AllTasksError`'s message.
4. A forced 401 still ends at `/login` with the redirect search param.
5. `make web-check` and `make web-build` pass.

## Open questions

1. A not-found page for a signed-out user: show it bare (root-level), or redirect to `/login`? I
   assume bare, because `_authed` already redirects any signed-out visit to its own routes.
2. Is dropping the per-dialog boundaries acceptable (decision 2)? The idea list had them.
3. Should Try again also be offered on `AllTasksError`? I assume no: a 403 does not heal on retry.
