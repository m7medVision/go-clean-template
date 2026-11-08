# Implementation plan: web end-to-end tests

Spec: [web-e2e.md](web-e2e.md) · Tasks: [web-e2e-todo.md](web-e2e-todo.md) · Map: [web.md](web.md)

On `feat/web-e2e` (off `feat/web-tests`), three commits: the suite with its Makefile target; the CI
job; the docs. Run against `vp dev` proxying to the real app and worker (Postgres, Valkey, Mailpit
from `make compose-up`; SSO off). The CI job is not in `ci-ok`'s needs, so it blocks nothing
(decision 5 of the spec). The `web-e2e` job is written to mirror the integration job and has not
been run on GitHub.
