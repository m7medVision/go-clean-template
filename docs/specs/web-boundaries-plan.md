# Implementation plan: web import boundaries

Spec: [web-boundaries.md](web-boundaries.md) · Tasks: [web-boundaries-todo.md](web-boundaries-todo.md) · Map: [web.md](web.md)

## Overview

The first of six `web/` modules, one PR, four tasks, one commit each:
- first the one file that breaks the rules moves;
- then the rules go in, proven on throwaway violations;
- then the docs.

## Order, and why

```
1. move user/api.ts         profileQuery to auth/profile.ts; user/ deleted
2. try overrides            do they work from vite.config.ts? decides rule vs test (no commit)
3. the rules                lint overrides (or the fallback test); proven on violations
4. docs                     docs/web.md: layers, folder map
```

- **Task 1 goes before the rules** so the rules land on a tree that passes: the commit that turns
  them on is green.
- **Task 2 is a check, not a commit.** The spec tested Oxlint with standalone config files; the
  open risk is that `lint.overrides` in `vite.config.ts` behaves differently. Its answer picks
  between the lint rule and the Vitest fallback before task 3 is written.

## Commits

| Task | Subject |
| --- | --- |
| 1 | `refactor(web): move the profile query from user/ to auth/` |
| 3 | `build(web): enforce the import layers with lint` |
| 4 | `docs(web): the layers and the folder map` |

`refactor`, `build` and `docs` release nothing.

## Gates

- **G1** `make web-format` and `make web-check` pass.
- **G2** `make web-build` passes (the move touches the router's lazy chunks).
- **G3** each rule fails on its throwaway violation and passes once it is removed.

## Risks

| Risk | Mitigation |
| --- | --- |
| `lint.overrides` is not read from `vite.config.ts` | task 2 finds out first; fallback is a Vitest test scanning `src/**/*.{ts,tsx}` imports, in `web/src/boundaries.test.ts` |
| The `#/` alias is read as a comment or glob in a pattern | `regex` patterns, not `group`: `^#/(…)/`, already checked |
| Relative imports slip past a `#/` regex | the spec's rule covers `#/` only; task 3 also greps for `from "\.\./` across layers and adds a pattern if any exists |
| The regenerated `routeTree.gen.ts` or `graphql/` trip the rules | both are in `ignorePatterns` already; task 3 checks |
