# Tasks: web error handling

Plan: [web-errors-plan.md](web-errors-plan.md) · Spec: [web-errors.md](web-errors.md)

Gates (G1–G3) are defined in the plan. Every task gets one Conventional Commit and its boxes ticked
here, and adds no comments to new code unless one is important. Sizes: S = 1–2 files, M = 3–5.

---

### 1. The components
- [x] `RouteError({ error, reset })` in `web/src/components/route-error.tsx`: heading "Something
      went wrong", `problemMessage(error)` in an element with `role="alert"`, a "Try again" button
      that calls `reset`.
- [x] `RouteNotFound()` in the same file: "Page not found" and a link back to `/tasks`.
- [x] `web/src/components/route-error.test.tsx`, `renderToStaticMarkup` like
      `tasks/admin-error.test.tsx`:
  - [x] a problem's `detail` is shown, with `role="alert"`;
  - [x] an error without a body shows the generic fallback;
  - [x] a plain `Error("boom")` does not leak its message or stack;
  - [x] the button renders;
  - [x] `RouteNotFound` links to `/tasks`.

**Files:** `components/route-error.tsx`, `route-error.test.tsx`. **Verify:** G1.
**Depends on:** none (the `web-boundaries` rules already allow `components/` to import `api/`).
**Size:** S

### 2. Turn them on
- [x] `router.tsx`: `defaultErrorComponent: RouteError`, `defaultNotFoundComponent: RouteNotFound`.
- [x] "Try again" resets what failed: `router.invalidate()` with the query error reset first; fall
      back to `queryClient.resetQueries` on the failed keys if the cached error is served again.
- [x] By hand (G3), run 2026-11-08 against a stub API (one Node file) behind `vp dev`, not the Go service. 500 gave the error view with the sidebar, and Try again with the stub healthy loaded the list. 403 gave `AllTasksError`. `UNAUTHENTICATED` gave `/login?redirect=%2Ftasks` with the token cleared:
  - [x] API stopped: `/tasks`, `/exchange` and `/profile` show the error view, sidebar present;
        API started, Try again loads the page (criterion 1);
  - [x] `/no-such-page` shows the not-found page with a link to `/tasks` (criterion 2; a full page, no sidebar, because unknown URLs match no `_authed` route);
  - [x] `/admin/tasks` as a non-admin still shows `AllTasksError` (criterion 3);
  - [x] a forced 401 ends at `/login` with `redirect` (criterion 4).

**Files:** `web/src/router.tsx`, and `components/route-error.tsx` if the reset needs a hook.
**Verify:** G1, G2, G3. **Depends on:** 1. **Size:** S

### 3. Docs
- [x] `docs/web.md`: an "Errors" paragraph under "Talking to the API": the router defaults, what
      Try again resets, that 401s sign out centrally, and that a mutation's failure is `FormError`'s.
- [ ] `docs/specs/README.md` and `web.md`: mark `web-errors` done once merged.

**Files:** `docs/web.md`, `docs/specs/*`. **Verify:** G1. **Depends on:** 2. **Size:** S
