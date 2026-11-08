# Implementation plan: web screen tests

Spec: [web-tests.md](web-tests.md) · Tasks: [web-tests-todo.md](web-tests-todo.md) · Map: [web.md](web.md)

On `feat/web-tests` (off `feat/web-boundaries`), four commits: the dependencies and helpers with the
first screen test; the tasks flows; the sign-in and exchange flows; the docs. MSW 3 differs from the
spec's assumptions (no `graphql` helper in the shape expected, `onUnhandledFrame` instead of
`onUnhandledRequest`), so GraphQL operations are matched by name from the query text on a plain
`http.post("/graphql")` handler. The devtools components are mocked in the setup file: they cannot
unmount under happy-dom. Gates: `make web-check`; each flow fails on a deliberate break.
