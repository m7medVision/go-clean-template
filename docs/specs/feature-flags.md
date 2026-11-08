# Spec: feature flags with OpenFeature

Module id: `feature-flags` · fourth of four (see [README.md](README.md#observability-and-messaging)) ·
Depends on `task-attributes` · Status: **spec written 2026-11-08, awaiting review**

## Objective

A feature can ship dark and be turned on without a deploy, decided by the backend and followed by
the web client. The template shows it on one real feature, task attributes. Done means:

- flags are read through the **OpenFeature** Go SDK, never a vendor SDK;
- in development, **GO Feature Flag** reads `flags.yaml` from the repo; edit the file, the flag
  changes;
- in production there is no provider in the template: every flag answers its default written in
  code, and the docs show where a real provider plugs in;
- the web client asks the backend for the caller's flags and never evaluates one itself;
- `task-attributes` is off by default: PATCH answers 404, reads leave out `attributes`, the web hides
  the columns and the dialog section; on, everything from `task-attributes` works.

## Design (decided)

1. **OpenFeature as the abstraction**, no repo port on top of it. The client is built once in
   `internal/app` and handed to handlers as a dependency.
2. **Typed flags with code defaults** in `internal/entity/flag.go`:

   ```go
   type Flag struct {
   	Key     string
   	Default bool
   }

   func FlagTaskAttributes() Flag { return Flag{Key: "task-attributes", Default: false} }
   ```

3. **Provider by presence:** `FEATURE_FLAGS_FILE` set (in `.env.example`, so `make dev`) → GO
   Feature Flag on that file; unset → no provider, defaults. No toggle.
4. **Checked in handlers**, never in controllers: the attributes command and the task queries.
   The evaluation context is the user ID.
5. **The web reads flags from the backend:** a `GetFlags` query, `GET /v1/flags` and GraphQL
   `flags`, answering `{ "task-attributes": true }` for the caller; TanStack Query, one cache entry.
6. **Every flag has an owner and is removed after rollout**; the docs say so.

## Configuration

| Env var | Default | Read by |
| --- | --- | --- |
| `FEATURE_FLAGS_FILE` | empty: defaults only | the app (`flags.yaml` in `.env.example`) |

## Project structure

| Where | Change |
| --- | --- |
| `internal/entity/flag.go` | `Flag`, the flags list |
| `flags.yaml` | the development flags |
| `pkg/flags/` | build the OpenFeature client, with GO Feature Flag when a file is given |
| `internal/usecase/flags/query/get_flags.go`, `register.go` | the query |
| `internal/usecase/task/...` | the `task-attributes` checks |
| `internal/controller/restapi/v1/flags/`, `graphql/schema/flags.graphqls`, `graphql/flags/` | the transports (no gRPC) |
| `internal/app/flags.go`, `modules.go`, `config/` | wiring |
| `web/src/flags/api.ts`, `tasks/columns.tsx`, `tasks/task-dialogs.tsx` | read flags, hide UI |
| `docs/feature-flags.md`, `docs/api.md`, `docs/configuration.md` | how to add, check and remove a flag |

## Testing strategy

- **Handlers:** with OpenFeature's in-memory provider: flag off → `ErrTaskNotFound`-shaped answer
  and no attributes; on → as `task-attributes`.
- **`pkg/flags`:** no file → defaults; a file → its values.
- **Transports:** `GET /v1/flags` and `flags` for a caller.
- **Web:** the columns and dialog section hidden when off.
- **Integration:** the dev `flags.yaml` turns `task-attributes` on, so its tests run.

## Boundaries

- **Always:** a default in code for every flag; checks in handlers.
- **Ask first:** a production provider; flags for anonymous users; a second flag.
- **Never:** permissions as flags; a vendor SDK outside `pkg/flags`; the browser deciding a flag.

## Success criteria

1. No `FEATURE_FLAGS_FILE`: `task-attributes` is off everywhere, with no error.
2. With the file and the flag on, `task-attributes`' success criteria pass.
3. `GET /v1/flags` and GraphQL `flags` answer the same values the handlers use.
4. Checks green.

## Open questions

1. Flag off: PATCH answers 404, or 403 / 501?
2. Does GO Feature Flag reload `flags.yaml` while running, or on restart only?
