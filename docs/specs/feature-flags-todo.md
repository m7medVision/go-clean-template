# Tasks: feature flags with OpenFeature

Plan: [feature-flags-plan.md](feature-flags-plan.md) · Spec: [feature-flags.md](feature-flags.md)

Done for every task: `make format`, `make linter-golangci` and `make test` pass. Assumes the review
picks in the plan; do not start until they are confirmed.

## 1. Flag client, wired, nobody evaluates — M

- [ ] `internal/entity/flag.go`: `Flag`, `FlagTaskAttributes()`, `Flags()`, `FlagValues`; `ErrFeatureDisabled` (kind not found).
- [ ] `pkg/flags`: `New(ctx, file, l)`: no file → no provider; a file → GO Feature Flag in-process with a file retriever; registers the OTel traces hook and `logHook` (`Error` stage only, one `Warn`).
- [ ] Tests: no file → defaults; a file → its values; a failing provider → the default and one `Warn`; an evaluation → an event on the active span.
- [ ] `config` field, `.env.example`, `flags.yaml`, `docker-compose.yml`, `ci.yml`; `internal/app/flags.go` builds the client.
- [ ] Verify the reload behaviour (spec open Q2) and record it in the spec.
- [ ] `make deps-audit` is clean.

## 2. SetTaskAttributes honours the flag — S (dep 1)

- [ ] `SetTaskAttributesHandler.Flags`; off → `ErrFeatureDisabled`; `task.Deps.Flags` and `register.go`.
- [ ] Tests with the in-memory provider: off → `ErrFeatureDisabled` and no write; on → unchanged.
- [ ] REST lists 404 for the op; the existing tests still pass.

## 3. Task reads honour the flag — M (dep 1)

- [ ] `GetTask`, `ListTasks`, `ListAllTasks`: off → no `Attributes.Get` call, attributes empty.
- [ ] Mock tests: off → the attributes store is not called; on → it is merged as before.
- [ ] GraphQL `attributes` answers an empty object while off.

## Checkpoint: after 1–3

- [ ] `make compose-up-integration-test` green with `flags.yaml` on.
- [ ] Manual: with `task-attributes: false` in `flags.yaml`, PATCH answers 404 and reads carry no attributes, after the reload.

## 4. GetFlags query — S (dep 1)

- [ ] `internal/usecase/flags/query/get_flags.go` and `register.go`; a module in `internal/app/modules.go`.
- [ ] Test: answers the caller's values from the in-memory provider.

## 5. REST and GraphQL transports — M (dep 4)

- [ ] `GET /v1/flags` (`restapi/v1/flags/`), with Security; `make openapi`.
- [ ] `graphql/schema/flags.graphqls`, `graphql/flags/`, the resolver alias; `make graphql`.
- [ ] Transport tests against fake handlers; an integration test where both answer the same values.

## 6. Web hides the UI — M (dep 5)

- [ ] `make web-api`, `make web-graphql`; a shared `flagsQueryOptions` (not a domain: the import rule).
- [ ] Task columns and the dialog section hidden while off; loader `ensureQueryData`.
- [ ] Screen tests: off → hidden, on → shown; `make web-check`.

## 7. Docs — S (dep 1–6)

- [ ] `docs/feature-flags.md` (add, check and remove a flag; owner and removal; where a production provider plugs in).
- [ ] `docs/api.md`, `docs/configuration.md`, `docs/cqrs.md`, the `docs/architecture.md` packages table, CLAUDE.md.
- [ ] The spec marked built; the README map updated.

## Checkpoint: complete

- [ ] `make pre-commit` and `make compose-up-integration-test` green.
- [ ] The spec's success criteria 1–4 checked, adjusted for the confirmed default.
