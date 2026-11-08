# Spec: web role gate

Module id: `web-authz` · third of six (see [web.md](web.md)) · Depends on `web-boundaries` ·
Status: **implemented 2026-11-08 on `feat/web-authz` under the /goal directive, not separately approved; see web-authz-todo.md for what is open**

Facts below were checked in `web/src` on 2026-11-08.

## Objective

Role checks in the UI go through one component, so a screen that is for admins says so in one place
and a non-admin never sees the part they cannot use. The server stays the only enforcement. Done
means:

- `<Authorization allowedRoles={["admin"]}>` renders its children only for a token carrying one of
  the roles, and an optional `fallback` otherwise;
- the sidebar's admin link and the `/admin/tasks` route use it;
- a non-admin who opens `/admin/tasks` by URL still sees the server's refusal (`AllTasksError`),
  as today.

## What exists today

- `auth/roles.ts`: `tokenRoles(token)` decodes the `roles` claim without verifying it; `isAdmin`.
- One caller of `isAdmin`: `components/app-sidebar.tsx:50`, which adds the admin links.
- `/admin/tasks` has no UI check; the API answers 403 and `AllTasksError` shows it.
- No screen compares a record's owner to the caller: the task list is the caller's own, the admin
  list is read-only, and the server refuses another user's task.

## Decisions (where I differ from the idea list)

1. **No ownership policy helper.** bulletproof-react's "only the owner can edit" has no caller here:
   the API returns only the caller's tasks to edit. A helper without a caller is speculation; add it
   with the first screen that needs it.
2. **The route keeps the server's refusal.** Hiding `/admin/tasks` from a non-admin in the UI would
   also hide the one honest message; the gate guards navigation and controls, not the URL.
3. **One generic component, roles as strings.** `allowedRoles: string[]`, matched against
   `tokenRoles`. No role enum: the server owns the list.

## Behaviour

| Case | Result |
| --- | --- |
| token has any of `allowedRoles` | children |
| no token, no roles, or none match | `fallback` (default: nothing) |
| token that cannot be decoded | as no roles |
| roles change (sign in as another user) | re-evaluated on render: the token is read each render |

## Project structure

| File | Change |
| --- | --- |
| `web/src/components/authorization.tsx` | new: `Authorization({ allowedRoles, fallback?, children })`, reading `auth.token()` |
| `web/src/components/authorization.test.tsx` | new: `renderToStaticMarkup`, a token built like `roles.test.ts` does |
| `web/src/components/app-sidebar.tsx` | the admin links inside `<Authorization allowedRoles={["admin"]}>` instead of the inline `isAdmin` ternary |
| `docs/web.md` | one sentence under "Screens" |

`components/` imports `auth/` (shared), which the layer rules allow. `isAdmin` stays, used by
`Authorization`'s callers only if one needs a boolean; if none does after the change, remove it.

## Commands

`make web-format`, `make web-check`, `make web-build`.

## Testing

- Unit: renders children for a matching role, for one of several; renders `fallback` for no token,
  empty roles, a different role and a malformed token.
- By hand once: sign in as a user and as an admin; only the admin sees the sidebar link; a user
  opening `/admin/tasks` sees `admin role required`.

## Boundaries

- Always: treat the check as presentation; keep the server's 403 reachable.
- Ask first: adding a role name to the UI; a `PBAC` or ownership helper.
- Never: use the gate for anything the server does not also enforce; verify the token's signature
  client-side.

## Success criteria

1. The sidebar admin link is rendered through `Authorization`, with the same visible result as now.
2. The unit cases above pass.
3. `/admin/tasks` as a non-admin still shows `admin role required`.
4. `make web-check` and `make web-build` pass.

## Open questions

1. This module is small (one component, one caller). Do you want it, or fold it into `web-errors`?
   I recommend keeping it separate but doing it last of the three that follow `web-boundaries`.
2. Agree with leaving the ownership helper out (decision 1)?
