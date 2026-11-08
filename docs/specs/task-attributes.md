# Spec: Task attributes in MongoDB

Module id: `task-attributes` · second of two (see [README.md](README.md#mongodb)) · Depends on
`mongo` · Status: **approved 2026-11-09**

Paths below are checked against `feat/valkey` at `25a746b` on 2026-11-08.

## Objective

A task gets **attributes**: typed, validated fields that product adds and changes often (to start:
`priority`, `estimate`, `labels`). They live in MongoDB, one document per task, while the task
itself stays event-sourced in Postgres. This is the template's example of storing data in a
document store instead of Postgres, behind a port, inside an existing domain.

- `PATCH` a task's attributes, on REST, gRPC and GraphQL. A field left out stays as it is, `null`
  clears it, a value sets it.
- Every task read (get, list, admin list; REST, gRPC, GraphQL) returns the task's `attributes`
  inline. A task with no document has empty attributes.
- Deleting a task deletes its document.
- Adding an attribute is a change to one Go struct, its validation and the transport DTOs: no
  schema migration, no event upcast.

**Who it is for:** a developer starting from the template who needs to see where and how
fast-changing data leaves the event-sourced Postgres path, and what that costs.

## Design (decided)

1. **Typed, not free-form.** `internal/entity/mongo/task/attributes.go` (package `task`, imported as
   `mongotask`) holds `Attributes` and `AttributesPatch`, each field with its own type and rule.
   No `map[string]any` reaches a handler.
2. **Written directly to Mongo, no events.** `SetTaskAttributes` checks the task exists and is the
   caller's through `repo.TaskReadModel` (`ReadableBy`), then writes through
   `repo.TaskAttributesStore`. It returns an `entity.Written` with no events, so the wait step does
   not wait. This is the one documented exception to "writes go through aggregates": no history,
   and Mongo must be backed up, because nothing rebuilds it.
3. **Merge semantics (PATCH).** `AttributesPatch` has a three-state `Field[T]` per attribute
   (absent / clear / set); the store turns it into one `$set` / `$unset` update with upsert.
4. **Reads merged in.** `GetTask` makes one `Attributes.Get`; `ListTasks` and `ListAllTasks` one
   `Attributes.GetMany(ids)` per page. They answer `entity.TaskWithAttributes` (`entity.Task`
   embedded, plus `Attributes`), so `entity.Task(row)` from sqlc keeps compiling.
5. **Delete in the same command.** `DeleteTask` deletes the task through the aggregate first, then
   the document. A Mongo failure is logged and the command still succeeds: the worst case is an
   unreachable orphan document.
6. **No filtering or sorting by attributes** in this module.

```go
// internal/entity/mongo/task/attributes.go
type Priority string

type Attributes struct {
	Priority *Priority `json:"priority,omitempty" bson:"priority,omitempty"`
	Estimate *float64  `json:"estimate,omitempty" bson:"estimate,omitempty"`
	Labels   []string  `json:"labels,omitempty"   bson:"labels,omitempty"`
}

type Field[T any] struct {
	Set   bool
	Value *T
}

type AttributesPatch struct {
	Priority Field[Priority]
	Estimate Field[float64]
	Labels   Field[[]string]
}
```

```go
// internal/repo/task.go
type TaskAttributesStore interface {
	Get(ctx context.Context, taskID string) (mongotask.Attributes, error)
	GetMany(ctx context.Context, taskIDs []string) (map[string]mongotask.Attributes, error)
	Patch(ctx context.Context, taskID string, p mongotask.AttributesPatch) error
	Delete(ctx context.Context, taskID string) error
}
```

Rules to start with (constants in the entity package, pinned by the transports' tests):
`priority` ∈ `low|medium|high`; `estimate` in hours, `0.5`–`200`; `labels` at most 10, each 1–30
characters, no duplicates.

## Transports

| Transport | Operation | Three-state |
| --- | --- | --- |
| REST | `PATCH /v1/tasks/{id}/attributes`, `Security` declared, answers the task with attributes | body decoded so a missing key and `null` differ; each field still documented with its type in OpenAPI |
| gRPC | `UpdateTaskAttributes(id, attributes, google.protobuf.FieldMask)` | a field in the mask and empty clears it |
| GraphQL | `updateTaskAttributes(id: ID!, input: TaskAttributesInput!): Task!`; `Task.attributes: TaskAttributes!` | `graphql.Omittable[T]` per input field |
| Web | the task dialog edits attributes; the table shows a `priority` and `labels` column | the form sends only dirty fields, "clear" sends `null` |

## Project structure

| Where | What |
| --- | --- |
| `internal/entity/mongo/task/` | `attributes.go`, `attributes_test.go` |
| `internal/entity/task.go` | `TaskWithAttributes` |
| `internal/repo/task.go` | `TaskAttributesStore`, its `go:generate`, then `make mock` |
| `internal/repo/mongo/task/` | `attributes.go` (the store: `tasks_attributes` collection, `_id` = task ID), `attributes_test.go` |
| `internal/usecase/task/command/` | `set_task_attributes.go` + test; `delete_task.go` gets `Attributes` and `Logger` |
| `internal/usecase/task/query/` | `get_task.go`, `list_tasks.go`, `list_all_tasks.go` read attributes; tests |
| `internal/usecase/task/register.go` | `Deps.Attributes`, `Deps.Logger`; the new handler's line |
| `internal/app/task.go` | the Mongo store into `Deps` |
| `internal/controller/restapi/v1/task/` | `update_task_attributes.go`, the attributes in `response.go`, tests |
| `internal/controller/grpc/v1/task/`, `docs/proto/v1/task.proto` | the RPC, the message, `response.go`, tests; `make proto-v1` |
| `internal/controller/graphql/` | `schema/update_task_attributes.graphqls`, attributes types in `schema/task_types.graphqls`, `task/update_task_attributes.go`, tests; `make graphql` |
| `web/src/tasks/` | `schema.ts`, `api.ts`, `columns.tsx`, `task-dialogs.tsx`; `make web-api`, `make web-graphql` |
| `integration-test/task_attributes_test.go` | over REST, gRPC and GraphQL |
| `docs/api.md`, `docs/cqrs.md`, `docs/architecture.md`, `CLAUDE.md` | the operation rows; the exception to "writes go through aggregates"; `internal/entity/mongo` and `internal/repo/mongo` in the Packages table and **Where code goes** |

Templates under `tools/gen/templates` copy `task` files. **`make new-domain` never writes a Mongo
store** (decided 2026-11-09): the templates keep the pre-attributes shape of each file attributes
touch, and `docs/generator.md` and CLAUDE.md's "templates copy `task`" rule name attributes as the
one part of `task` the generator does not copy.

## Testing strategy

- **Entity:** table-driven `Validate()` for every rule and every patch state; `Changes()` produces
  the expected `$set`/`$unset`.
- **Command handlers:** `SetTaskAttributes` against `memstore` plus a mock `TaskAttributesStore`:
  not found, forbidden, invalid, written. `DeleteTask` deletes the document and still succeeds when
  the store fails.
- **Query handlers:** with `repomock`: attributes merged, missing document is empty, one `GetMany`
  per page.
- **Store:** against the stack's Mongo in `integration-test/` (no Mongo in `make test`): patch
  absent/clear/set, `GetMany`, delete idempotent.
- **Transports:** the REST, gRPC and GraphQL tests show missing vs `null` vs value reach the patch
  unchanged.
- **Integration:** create a task, patch attributes, read it back on each transport, delete it,
  confirm the document is gone.
- **Web:** `schema.test.ts` for the Zod rules.

## Boundaries

- **Always:** the repo's checks (`make format`, `make linter-golangci`, `make test`; `make web-api`,
  `make web-graphql`, `make web-check`; `make gen-check`, `make gen-e2e` if a mirrored file
  changes); `docs/api.md` rows in the same series.
- **Ask first:** a new attribute beyond the three; filtering or sorting by attributes; events for
  attribute changes; attributes on another domain.
- **Never:** `map[string]any` past the transport boundary; the Mongo driver imported by `usecase`
  or a controller; a projector writing to Mongo; a read-model table for attributes in Postgres.

## Success criteria

1. `PATCH` with `{"priority":"high"}` leaves `estimate` and `labels` unchanged; `{"estimate":null}`
   clears it; an invalid value answers 422 / `InvalidArgument` / `BAD_USER_INPUT` and writes nothing.
2. Another user's task answers 403 / `PermissionDenied` / `FORBIDDEN` and writes nothing.
3. `GET /v1/tasks/{id}`, the lists, gRPC and GraphQL return `attributes`; a task without a document
   returns empty ones.
4. A list page makes one Mongo call, whatever its size.
5. Deleting a task removes its document; with Mongo failing, the delete still answers 204 and logs
   the failure.
6. The web table shows priority and labels; the dialog edits them.
7. All checks above pass; CI is green.

## Decided (conversation of 2026-11-08)

Example inside `task`, no new domain · the generator never writes a Mongo store · all three
transports plus the web · PATCH answers the task with its attributes, as `updateTask` does · the
store tested for real only in `integration-test/`, as the Postgres stores are · typed attributes in `internal/entity/mongo/task/` · direct
write, no events · merged into every task read · Mongo required · delete in the same command, task
first, Mongo failure logged · no filtering · PATCH merge with three states.

## Open questions

None.
