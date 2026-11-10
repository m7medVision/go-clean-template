# Implementation plan: feature flags with OpenFeature

Spec: [feature-flags.md](feature-flags.md) · Tasks: [feature-flags-todo.md](feature-flags-todo.md) · Map: [README.md](README.md#observability-and-messaging)

## Overview

One branch, `feat/feature-flags`, one PR. The flag client goes in first and nothing reads it. The
`task-attributes` checks come next, then the `GetFlags` query and its transports, then the web,
then the docs. Seven tasks, one commit each.

Built 2026-11-10 on the review picks below, confirmed by the request to build every task. Tasks 2
and 3 landed as one commit (`feat(task): turn task attributes off with their flag`): they share
`register.go`. Tasks 4 and 5 likewise. The flags file is `config/flags.yaml`, not the root: the
image already copies `config/`.

| # | Review point | Pick the plan assumes | Tasks it changes |
| --- | --- | --- | --- |
| 1 | `task-attributes` default | **on** (a kill switch: the feature is already merged) | 1, 3, 6 |
| 2 | GraphQL `attributes` when off | an **empty object**; the schema stays non-null | 3 |
| 3 | Flags shape | typed `entity.FlagValues{TaskAttributes bool}`, `taskAttributes` on both transports | 4, 5, 6 |
| 4 | PATCH when off | 404 from a new sentinel `ErrFeatureDisabled` (kind not found) | 2 |
| 5 | Reload | GO Feature Flag polls the file: checked in task 1, the answer goes in the spec | 1 |
| 6 | CI / compose | `FEATURE_FLAGS_FILE` in `ci.yml` and `docker-compose.yml` | 1 |

## Order, and why

```
1. pkg/flags + entity.Flag + wiring    client built, hooks registered, nobody evaluates
2. SetTaskAttributes check             PATCH honours the flag
3. task queries check                  reads skip Mongo when off
4. GetFlags query                      the answer the web reads
5. REST GET /v1/flags, GraphQL flags   transports (no gRPC)
6. web: read flags, hide UI            depends on 5's generated types
7. docs                                feature-flags.md, api, configuration, cqrs, architecture
```

Task 1 makes the client and wires it into every stack before any handler depends on it. The
handlers (2 and 3) come before the transports, so `GET /v1/flags` reports exactly what the handlers
already use (success criterion 3). The web comes after 5 because it needs the generated REST types.

## Commits

| Task | Subject |
| --- | --- |
| 1 | `feat(flags): build an OpenFeature client with GO Feature Flag in development` |
| 2 | `feat(task): answer 404 to setting attributes while the flag is off` |
| 3 | `feat(task): leave attributes out of reads while the flag is off` |
| 4 | `feat(flags): add the GetFlags query` |
| 5 | `feat(flags): expose the caller's flags over REST and GraphQL` |
| 6 | `feat(web): hide task attributes while the flag is off` |
| 7 | `docs: feature flags` |

## Gates

Every task: `make format`, `make linter-golangci`, `make test`. Tasks 4–6 add `make openapi`,
`make graphql`, `make web-api`, `make web-graphql` and `make web-check` where they apply. Before the
PR, `make pre-commit` and `make compose-up-integration-test`, run in the isolated stack (not beside
`make dev`), with heavy runs under the shared heavy lock.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| The contrib OTel hook or the in-process provider's API differs from what was sketched | Med | Task 1 checks the current docs (context7) before writing any code |
| The provider pulls in a large dependency tree or one with a known vulnerability | Med | `make deps-audit` in task 1; if the tree is too big, fall back to OpenFeature's in-memory provider fed from YAML |
| Every task read now evaluates a flag | Low | In-process evaluation, no network; measure once in task 3 |
| `dupl` between the REST and GraphQL flag handlers | Low | Both are thin; map through one `entity.FlagValues` |
| The flag stays on forever | Low | `docs/feature-flags.md` names an owner and a removal date |

## Not in this plan

A production provider, flags for anonymous users, a second flag, flag metrics, and a test that
fails once a removal date passes. That last one is a gap raised in review and not yet decided.
