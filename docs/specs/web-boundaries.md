# Spec: web import boundaries

Module id: `web-boundaries` · first of six (see [web.md](web.md)) · Depends on nothing ·
Status: **done: merged 2026-11-08 (#53), released in v0.5.0**

Facts below were checked in `web/` on 2026-11-08 (Oxlint 1.89.0 in `web/node_modules`).

## Objective

The layering of `web/src` is enforced by `make web-check`, not by habit. A change that makes a
domain import another domain, or shared code import a domain or a route, fails the lint step. Users
see no change. Done means:

- the rules below are in the lint config and `make web-check` fails on a violation;
- the two imports that break them today are gone;
- `docs/web.md` has the folder map and the rules, so a contributor knows where code goes.

## Layers and rules

| Layer | Folders | May import |
| --- | --- | --- |
| shared | `api`, `auth`, `components`, `hooks`, `lib` (and later `testing`) | shared only |
| domain | `tasks`, `exchange`, `passkey`, `sso` | shared, and its own folder |
| routes | `routes` | everything |

`graphql/` (generated) and `api/schema.gen.ts` count as shared. Nothing imports `routes/`.

## What breaks them today

Two imports, both of `profileQuery` from `user/api.ts`:

- `src/components/app-sidebar.tsx:15` (shared → domain)
- `src/passkey/passkeys-card.tsx:33` (domain → domain)

`user/` has no other file and three importers (the two above and `routes/_authed/profile.tsx`): it is
shared code wearing a domain's name. Fix: move `user/api.ts` to `src/auth/profile.ts` (the signed-in
user's profile belongs with the session) and delete `user/`. This drops `user/` from the tree in
[web.md](web.md)'s discussion; no behaviour changes.

## How it is enforced

Oxlint's built-in `no-restricted-imports` with `regex` patterns, in per-folder `overrides` in the
`lint` block of `web/vite.config.ts`. Checked by hand: a regex pattern flags `#/user/…` imports, and
override globs must be written `**/passkey/**` (a bare `src/…` glob matched nothing when the config
was outside `web/`). Sketch:

```ts
lint: {
  overrides: [
    { files: ["**/tasks/**"], rules: { "no-restricted-imports": ["error", { patterns: [
      { regex: "^#/(exchange|passkey|sso|routes)/", message: "a domain never imports another domain or a route" },
    ] }] } },
    // one block per domain, and one for the shared folders (any domain, and routes)
  ],
}
```

One rule per folder is more text than a single rule, but it needs no plugin and no new dependency.
A Vitest test that scans imports is the fallback if `overrides` cannot be set through `vite.config.ts`;
the plan decides after trying it.

## Commands

```
make web-check     # format check, lint (the rules), type check, tests
make web-format    # format and autofix
```

No new target.

## Project structure

| File | Change |
| --- | --- |
| `web/vite.config.ts` | the `overrides` in `lint` |
| `web/src/user/api.ts` | moved to `web/src/auth/profile.ts`; the three importers updated |
| `docs/web.md` | the layer table above and the folder map |

## Testing

- Each rule is proven once by a throwaway violation (a domain importing a domain, a shared file
  importing a domain, a shared file importing a route): `make web-check` must fail on each, then
  pass after removal. Recorded in the PR, not committed.
- `make web-check` passes on the final tree.

## Boundaries

- Always: run `make web-format` and `make web-check`; keep imports on the `#/` alias.
- Ask first: adding any dependency; splitting `tasks/` into subfolders; moving more code than
  `user/api.ts`.
- Never: a `// oxlint-disable` on these rules; changes under Go packages.

## Success criteria

1. `make web-check` fails when a domain imports a domain, when shared code imports a domain, and
   when anything imports `routes/`.
2. `make web-check` passes on the PR's tree.
3. No file under `src/user/`; `profileQuery` is imported from `#/auth/profile`.
4. `docs/web.md` states the three layers and their import rules.

## Open questions

1. `user/api.ts` → `auth/profile.ts`: ok, or a different home (`api/profile.ts`)?
2. You did not answer whether to split `tasks/` (now 11 files). I assume no, and leave it for a later
   change; say so if you want it here.
3. Questions 2 and 3 of [web.md](web.md) (error-tracking timing; `web-e2e` blocking in CI) stay open
   until those modules are specced.
