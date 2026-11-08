# Tasks: web screen tests

Plan: [web-tests-plan.md](web-tests-plan.md) · Spec: [web-tests.md](web-tests.md)

- [x] Dependencies (`happy-dom`, `@testing-library/react`, `user-event`, `dom`, `msw`), `testing/` helpers, `test` config, first screen test.
- [x] Tasks: list, add, move, delete.
- [x] Sign-in: success, wrong password, rate limit. Exchange: history, convert, refusal.
- [x] Deliberate breaks, 2026-11-08: a changed transition rule fails the move test; `problemMessage` ignoring `detail` fails the two sign-in refusal tests; dropping the history invalidation fails the convert test; renaming a field in a handler body fails type checking.
- [x] Suite time 2.3 s (was 0.6 s), under the 10 s budget.
- [x] `docs/web.md` "Testing" section and a line in `CLAUDE.md`.
