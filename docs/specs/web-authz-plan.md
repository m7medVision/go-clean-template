# Implementation plan: web role gate

Spec: [web-authz.md](web-authz.md) · Tasks: [web-authz-todo.md](web-authz-todo.md) · Map: [web.md](web.md)

Two tasks on `feat/web-authz` (off `feat/web-boundaries`), one commit each:
1. the `Authorization` component, its tests, and the sidebar using it (a `refactor`: same visible result);
2. the docs sentence.

`isAdmin` had one caller (the sidebar). With the gate in place it has none, so it and its tests are
removed in task 1, as the spec says. No dependency, no config. Gates: `make web-format`,
`make web-check`, `make web-build`, and the by-hand sidebar check.
