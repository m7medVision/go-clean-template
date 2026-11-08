# Tasks: web role gate

Plan: [web-authz-plan.md](web-authz-plan.md) · Spec: [web-authz.md](web-authz.md)

### 1. The component and the sidebar
- [x] `Authorization({ allowedRoles, fallback?, children })` in `web/src/components/authorization.tsx`.
- [x] Tests: a matching role, one of several, no token, no roles claim, a different role, a malformed token.
- [x] The sidebar's admin link inside it, through a shared `NavItem`; `isAdmin` and its tests removed.
- [x] By hand, 2026-11-08, `vp dev` with a stub API: a `["user"]` token shows `/tasks`, `/exchange`,
      `/profile`; a `["user","admin"]` token adds `/admin/tasks`.

**Verify:** `make web-check`, `make web-build`. **Size:** S

### 2. Docs
- [x] `docs/web.md`: the gate is named under "Screens".
