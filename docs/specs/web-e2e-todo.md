# Tasks: web end-to-end tests

Plan: [web-e2e-plan.md](web-e2e-plan.md) · Spec: [web-e2e.md](web-e2e.md)

- [x] `@playwright/test`, `web/e2e/playwright.config.ts`, `web/e2e/tasks.spec.ts`, `make web-e2e`, `.gitignore`; vitest limited to `src/**/*.test.*` so it does not pick up the spec.
- [x] Run 2026-11-08 against the real app and worker: 3 passes in a row (4.1 to 4.9 s each).
- [x] Deliberate break: renaming the "Add task" button fails the suite and keeps the trace.
- [x] CI job `runner / web / e2e`, not in `ci-ok`. **Not yet run on GitHub.**
- [x] `docs/web.md` "End-to-end test" and a pointer in `docs/development.md`.
- [ ] CI added time under 4 minutes (needs a real run).
