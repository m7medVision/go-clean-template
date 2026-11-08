# Tasks: web import boundaries

Plan: [web-boundaries-plan.md](web-boundaries-plan.md) · Spec: [web-boundaries.md](web-boundaries.md)

Gates (G1–G3) are defined in the plan. Every task gets one Conventional Commit (task 2 none) and its
boxes ticked here, and adds no comments to new code unless one is important. Sizes: S = 1–2 files,
M = 3–5.

---

### 1. Move the profile query to `auth/`
- [x] `git mv web/src/user/api.ts web/src/auth/profile.ts`; remove `web/src/user/`.
- [x] Update the three importers to `#/auth/profile`: `components/app-sidebar.tsx`,
      `passkey/passkeys-card.tsx`, `routes/_authed/profile.tsx`.
- [x] `grep -rn '#/user' web/src docs` finds nothing (`docs/web.md` mentions included).

**Files:** `auth/profile.ts`, the three importers. **Verify:** G1, G2. **Depends on:** none.
**Size:** S

### 2. Find out if `overrides` works from `vite.config.ts`
- [x] Add one `lint.overrides` block for `**/passkey/**` with a `regex` pattern; add a throwaway
      `import "#/tasks/status"` to a passkey file; `make web-check` must report
      `no-restricted-imports`.
- [x] Do the same for a shared folder (`**/components/**` importing a domain).
- [x] Revert the throwaways. Result: `lint.overrides` in `vite.config.ts` works (checked 2026-11-08); task 3 is the lint rule.

**Files:** none kept. **Verify:** the rule message appears. **Depends on:** 1. **Size:** S

### 3. The rules
- [x] Domain blocks, one per `tasks`, `exchange`, `passkey`, `sso`: forbid the other three domains
      and `routes`, with the message `a domain never imports another domain or a route`.
- [x] Shared block (`api`, `auth`, `components`, `hooks`, `lib`): forbid every domain and `routes`,
      with the message `shared code never imports a domain or a route`.
- [x] Nothing imports `routes/`: covered by the domain and shared blocks above (`main.tsx` and `router.tsx` import only the generated route tree), so no block of its own.
- [x] Prove each on a throwaway violation (G3): domain → domain, shared → domain, shared → route.
      Record the three results in the PR description.
- [x] (Not needed: task 2 passed.)
- [x] Relative cross-layer imports: none exist (`from "../` finds nothing), so no pattern.

**Files:** `web/vite.config.ts` (or `web/src/boundaries.test.ts`). **Verify:** G1, G3.
**Depends on:** 2. **Size:** S

### 4. Docs
- [x] `docs/web.md`: the layer table, the three rules, the folder map, and where `profileQuery`
      lives now.
- [x] `docs/specs/README.md` and `web.md`: mark `web-boundaries` done once merged.
- [x] `CLAUDE.md` **Frontend** section: one line pointing at the layers (it is the rules file).

**Files:** `docs/web.md`, `docs/specs/*`, `CLAUDE.md`. **Verify:** G1. **Depends on:** 3. **Size:** S
