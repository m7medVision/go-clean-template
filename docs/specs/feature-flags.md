# Spec: feature flags with OpenFeature

Module id: `feature-flags` · fourth of four (see [README.md](README.md#observability-and-messaging)) ·
Depends on `task-attributes` · Status: **approved 2026-11-10 with the review picks below; built on `feat/feature-flags`, awaiting review**

## Objective

A feature can ship dark and be turned on without a deploy, decided by the backend and followed by
the web client. The template shows it on one real feature, task attributes. Done means:

- flags are read through the **OpenFeature** Go SDK, never a vendor SDK;
- in development, **GO Feature Flag** reads `config/flags.yaml` from the repo; edit the file, the
  flag changes within 10 seconds;
- in production there is no provider in the template: every flag answers its default written in
  code, and the docs show where a real provider plugs in;
- the web client asks the backend for the caller's flags and never evaluates one itself;
- `task-attributes` is **on** by default (a kill switch: the feature already shipped). Off: PATCH
  answers 404 (`entity.ErrFeatureDisabled`), reads return empty `attributes` (GraphQL: an empty
  object, so the schema stays non-null), the web hides the columns and the dialog section; on,
  everything from `task-attributes` works.

## Design (decided)

1. **OpenFeature as the abstraction**, no repo port on top of it. The client is built once in
   `internal/app` and handed to handlers as a dependency.
2. **Typed flags with code defaults** in `internal/entity/flag.go`:

   ```go
   type Flag struct {
   	Key     string
   	Default bool
   }

   func FlagTaskAttributes() Flag { return Flag{Key: "task-attributes", Default: true} }
   ```

3. **Provider by presence:** `FEATURE_FLAGS_FILE` set (in `.env.example`, compose and so CI) → GO
   Feature Flag on that file; unset → no provider, defaults. No toggle.
4. **Checked in handlers**, never in controllers: the attributes command and the task queries.
   The evaluation context is the user ID.
5. **The web reads flags from the backend:** a `GetFlags` query, `GET /v1/flags` and GraphQL
   `flags`, answering a typed `entity.FlagValues` (`{ "taskAttributes": true }`) for the caller;
   TanStack Query, one cache entry.
6. **Every flag has an owner and is removed after rollout**; the docs say so.
7. **Evaluations are observed through OpenFeature hooks**, registered once in `pkg/flags`; handlers
   carry no logging or tracing code for flags (agreed 2026-11-10):
   - **Trace:** the traces hook from OpenFeature's contrib package (`hooks/open-telemetry`) adds
     each evaluation (key, variant, reason) as an event on the message's span, so a trace tells
     "flag off" from "task missing".
   - **Log:** our own hook implements only the `Error` stage: one `Warn` with the flag key and the
     error when the provider fails and the code default is used. `Warn`, not `Error`: the request
     still succeeds, and `Error` would reach GlitchTip on every evaluation. A successful evaluation
     is not logged: every task read evaluates the flag.
   - **Metrics:** none in this module. A counter per flag and variant comes with the first flag
     that needs a removal decision.

## Configuration

| Env var | Default | Read by |
| --- | --- | --- |
| `FEATURE_FLAGS_FILE` | empty: defaults only | the app (`config/flags.yaml` in `.env.example`, `/config/flags.yaml` in compose) |

## Project structure

| Where | Change |
| --- | --- |
| `internal/entity/flag.go` | `Flag`, the flags list |
| `config/flags.yaml` | the development flags (under `config/`, which the image already copies) |
| `pkg/flags/` | build the OpenFeature client, with GO Feature Flag when a file is given; the trace and log hooks |
| `internal/usecase/flags/query/get_flags.go`, `register.go` | the query |
| `internal/usecase/task/...` | the `task-attributes` checks |
| `internal/controller/restapi/v1/flags/`, `graphql/schema/flags.graphqls`, `graphql/flags/` | the transports (no gRPC) |
| `internal/app/flags.go`, `modules.go`, `config/` | wiring |
| `web/src/flags/api.ts`, `tasks/columns.tsx`, `tasks/task-dialogs.tsx` | read flags, hide UI |
| `docs/feature-flags.md`, `docs/api.md`, `docs/configuration.md` | how to add, check and remove a flag |

## Testing strategy

- **Handlers:** with OpenFeature's in-memory provider: flag off → `ErrTaskNotFound`-shaped answer
  and no attributes; on → as `task-attributes`.
- **`pkg/flags`:** no file → defaults; a file → its values; a failing provider → the default and
  one `Warn`; an evaluation → an event on the active span.
- **Transports:** `GET /v1/flags` and `flags` for a caller.
- **Web:** the columns and dialog section hidden when off.
- **Integration:** the dev `flags.yaml` turns `task-attributes` on, so its tests run.

## Boundaries

- **Always:** a default in code for every flag; checks in handlers.
- **Ask first:** a production provider; flags for anonymous users; a second flag.
- **Never:** permissions as flags; a vendor SDK outside `pkg/flags`; the browser deciding a flag.

## Success criteria

1. No `FEATURE_FLAGS_FILE`: `task-attributes` answers its default (on) everywhere, with no error.
2. With the file and the flag on, `task-attributes`' success criteria pass.
3. `GET /v1/flags` and GraphQL `flags` answer the same values the handlers use.
4. Checks green.

## Answered questions

1. Flag off: PATCH answers 404, through a new sentinel `entity.ErrFeatureDisabled` (kind not found).
2. GO Feature Flag polls the file while running (`pkg/flags` sets 10 s); checked 2026-11-10: an
   edit flipped the flag 10 s later, with no restart.
