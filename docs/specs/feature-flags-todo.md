# Tasks: feature flags with OpenFeature

Plan: [feature-flags-plan.md](feature-flags-plan.md) · Spec: [feature-flags.md](feature-flags.md)

Done for every task: `make format`, `make linter-golangci` and `make test` pass. Built 2026-11-10.

## 1. Flag client, wired, nobody evaluates — M

- [x] `internal/entity/flag.go`: `Flag`, `FlagTaskAttributes()`, `Flags()`, `FlagValues`; `ErrFeatureDisabled` (kind not found).
- [x] `pkg/flags`: `New(ctx, file, l)`: no file → no provider; a file → GO Feature Flag in-process with a file retriever; registers the OTel traces hook and `logHook` (`Error` stage only, one `Warn`).
- [x] Tests: no file → defaults; a file → its values; a failing provider → the default and one `Warn`; an evaluation → an event on the active span.
- [x] `config` field, `.env.example`, `flags.yaml`, `docker-compose.yml`, `ci.yml`; `internal/app/flags.go` builds the client.
- [x] Verify the reload behaviour (spec open Q2) and record it in the spec.
- [x] `make deps-audit`: 0 vulnerabilities reached by the code.

## 2. SetTaskAttributes honours the flag — S (dep 1)

- [x] `SetTaskAttributesHandler.Flags`; off → `ErrFeatureDisabled`; `task.Deps.Flags` and `register.go`.
- [x] Tests with the in-memory provider: off → `ErrFeatureDisabled` and no write; on → unchanged.
- [x] REST lists 404 for the op; the existing tests still pass.

## 3. Task reads honour the flag — M (dep 1)

- [x] `GetTask`, `ListTasks`, `ListAllTasks`: off → no `Attributes.Get` call, attributes empty.
- [x] Mock tests: off → the attributes store is not called; on → it is merged as before.
- [x] GraphQL `attributes` answers an empty object while off.

## Checkpoint: after 1–3

- [ ] `make compose-up-integration-test` green with `flags.yaml` on. Not run: Docker on the build host could not create network interfaces (`failed to add the host <=> sandbox pair interfaces: operation not supported`) before any test ran. `integration-test/flags_test.go` is written and vets.
- [x] Reload checked against a real file: an edit flipped the flag 10 s later. PATCH 404 and empty reads while off are covered by the handler tests, not by a running app.

## 4. GetFlags query — S (dep 1)

- [x] `internal/usecase/flags/query/get_flags.go` and `register.go`; a module in `internal/app/modules.go`.
- [x] Test: answers the caller's values from the in-memory provider.

## 5. REST and GraphQL transports — M (dep 4)

- [x] `GET /v1/flags` (`restapi/v1/flags/`), with Security; `make openapi`.
- [x] `graphql/schema/flags.graphqls`, `graphql/flags/`, the resolver alias; `make graphql`.
- [x] Transport tests against fake handlers; an integration test where both answer the same values.

## 6. Web hides the UI — M (dep 5)

- [x] `make web-api`, `make web-graphql`; a shared `flagsQueryOptions` (not a domain: the import rule).
- [x] Task columns and the dialog section hidden while off; loader `ensureQueryData`. The shared query is `web/src/api/flags.ts`; the MSW server answers the default flags unless a test overrides them.
- [x] Screen tests: off → hidden, on → shown; `make web-check`.

## 7. Docs — S (dep 1–6)

- [x] `docs/feature-flags.md` (add, check and remove a flag; owner and removal; where a production provider plugs in).
- [x] `docs/api.md`, `docs/configuration.md`, `docs/cqrs.md`, the `docs/architecture.md` packages table, CLAUDE.md.
- [x] The spec marked built; the README map updated.

## Checkpoint: complete

- [ ] `make pre-commit` and `make compose-up-integration-test` green: integration blocked as above.
- [x] The spec's success criteria 1–4 checked, adjusted for the confirmed default.
