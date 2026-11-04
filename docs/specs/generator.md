# Spec: domain and use-case generator

Module id: `generator` · PR 2 of 2 · Depends on `cqrs` · Status: **approved; plan in [generator-plan.md](generator-plan.md)**

Paths and shapes below are checked against master at `e952512`, where `cqrs` merged (#45). Each
template copies the `task` domain's file at the same path.

## Objective

Two Makefile commands that write the files every new domain or use case needs. Each file gets its
package line, imports, types, constructors and every function signature. Handler bodies are stubs
that return "not implemented". The result builds, passes lint and tests, and answers on every
transport from the first run; you write the bodies.

**Who it is for:** the same developer as `cqrs`, comparing this repo with the .NET template.

| | .NET `dotnet new ca-usecase` | `make new-usecase` / `make new-domain` |
| --- | --- | --- |
| Unit | one use case | one use case, or a whole domain with CRUD |
| Files | one: request, empty validator, handler that throws `NotImplementedException` | every layer: aggregate and events, commands and queries, read model, projector, REST, gRPC, GraphQL, tests |
| Wiring | none needed: reflection finds handlers at startup | edits the shared files itself, behind safety checks |
| Endpoint | written by hand | generated on all three transports |
| Template rot | template is compiling C# | golden tests, plus a CI job that generates a domain and builds it |

**Agreed in the interview:**

- A separate Go module in its own folder, never built into the app.
- Templates are `.tmpl` files rendered with `text/template`, the way `internal/repo/mailtmpl`
  renders email. No copying of `task`.
- It takes the name only, no field definitions.
- It edits the shared files, but checks every edit first and writes nothing if any check fails.
- No markers in the code.

## Commands

```
make new-domain  name=invoice                                     [transports=rest,grpc,graphql] [dry-run=1]
make new-usecase domain=task name=archive-task kind=command       [transports=rest,grpc,graphql] [dry-run=1]
make new-usecase domain=task name=count-tasks  kind=query
make gen-check      # tools/gen: format check, lint, unit and golden tests
make gen-e2e        # generate into a throwaway worktree, then build, lint and test it (CI)
```

`make new-domain` runs `go -C tools/gen run . new-domain --root $(CURDIR) …`. If the generator
wrote anything, it then runs the regenerators the output needs:

```
make proto-v1 graphql sqlc mock openapi web-api web-graphql format
go build ./...
```

It stops at the first failure and prints the undo command. For `new-domain`, `transports`
defaults to all three, since transports are independent by design. For `new-usecase`, it defaults
to the transports the domain already has: `user` has no GraphQL. `dry-run=1` prints every file it
would create and every insertion with its context, then runs every rule and fails if one would;
it writes nothing.

## Names

`name` is kebab-case and singular: `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`. From `line-item`:

| Used as | Becomes |
| --- | --- |
| Go type, proto message, GraphQL type | `LineItem` |
| Go package and folder | `lineitem` |
| File | `line_item.go` |
| Table | `line_items` |
| REST path | `/v1/line-items` |
| Stream | `lineitem-<uuid>` |

Plurals follow plain English rules (`-s`, `-es`, `-y` → `-ies`). `plural=` overrides them.

## What `make new-domain name=invoice` writes

A user-owned domain with CRUD: `CreateInvoice`, `UpdateInvoice`, `DeleteInvoice`, `GetInvoice` and
`ListInvoices`. The base fields are `id`, `user_id`, `created_at` and `updated_at`.

**The stub rule: handler bodies are stubs; everything they sit on is real.** Command, query and
event handlers return `entity.ErrNotImplemented`. The following are real working code:
- the aggregate and its events;
- registration;
- the projector and the read model for the base columns;
- transport handlers, DTO conversion and error mapping;
- the tests.

| Layer | Created |
| --- | --- |
| entity | `invoice.go` (`Invoice`, the read shape); `invoice_aggregate.go` (`InvoiceAggregate`, `InvoiceStreamID`, `Apply`, the behavior methods); `invoice_events.go` (`InvoiceCreated`, `InvoiceUpdated`, `InvoiceDeleted`, `invoiceEvents()`); `invoice_aggregate_test.go` |
| use case | `usecase/invoice/register.go` (`Deps`, `Register`); `command/create_invoice.go`, `update_invoice.go`, `delete_invoice.go`, each with its test against `internal/repo/memstore`, and `command_test.go` with what those share (a store holding one invoice, and the commands' `CommandName` and `ActorID`), which keeps `dupl` from finding the update and delete tests to be clones; `query/get_invoice.go`, `list_invoices.go` and `query_test.go` against `internal/repo/repomock` |
| repo | `persistent/invoice/invoice.go` (`NewReadModel`, returning `repo.InvoiceReadModel` wrapped in `newTraced`), `tracing.go`, `projector.go` (`NewProjector`, `Name`, `Tables`, `Handle`, `Project`), `projector_test.go` against a fake sqlc `Querier`; `queries/invoice.sql`. No `importer.go`: a new domain never had rows without events |
| schema | `internal/app/schema/invoices.hcl` (the base columns, no foreign keys) |
| REST | `restapi/v1/invoice/` `controller.go` (`NewRoutes`, the `problem` error mapping, `readBack`), `invoice.go` (create, list, get, update and delete under `/v1/invoices`, each listing 501 among its errors), `request.go` (the empty create and update bodies), `response.go` (`InvoiceList`), `fake_test.go`, `invoice_test.go`, `main_test.go` |
| gRPC | `docs/proto/v1/invoice.proto`; `grpc/v1/invoice/` `controller.go` (`New`, the error mapping), `invoice.go`, `request.go`, `response.go`, `response_test.go`, `invoice_test.go` |
| GraphQL | `schema/invoice.graphqls`; `graphql/invoice/` `resolver.go` (`Query`, `Mutation`, `New`), `query.go`, `mutation.go`, `fake_test.go`, `query_test.go`, `mutation_test.go` |
| integration | `integration-test/invoice_test.go` |

**Empty shapes.** Go structs and proto messages may be empty, so the create and update request
DTOs and messages are generated empty, as the place to add fields. GraphQL forbids empty input
and object types, so the generated mutations take only `id`, and the domain gets no `input.go`
until it has a field.

**Generated tests:**
- The use-case tests assert that each stub handler returns `entity.ErrNotImplemented`. Each one
  says, in a comment, that the first real implementation should replace it. That test going red
  is the intended signal.
- The transport tests send through real buses with fake handlers registered (`fake_test.go` for
  REST and GraphQL, fakes inside the test for gRPC), as every domain's do. They test what is real:
  decoding, encoding and the error mapping, including "not implemented".
- The aggregate and projector tests assert real behaviour.
- The integration test asserts today's answer end to end, against the stub handlers and the
  running worker: REST 501, gRPC `Unimplemented`, GraphQL `NOT_IMPLEMENTED`.

**Shared files it edits:**

| File | Edit | Found by |
| --- | --- | --- |
| `internal/entity/event.go` | `invoiceEvents()` in `EventTypes()` | the `slices.Concat` call in `EventTypes` |
| `internal/entity/errors.go` | `ErrInvoiceNotFound`, `ErrInvoiceForbidden` | the `var ( … )` block, before its closing paren |
| `internal/repo/contracts.go` | the `InvoiceReadModel` interface and the `InvoiceFilter` its `List` pages with | the `type ( … )` block, before its closing paren |
| `internal/app/buses.go` | `invoice.Register(…)` in `registerHandlers`; `persistInvoiceRepo.NewProjector(pg)` in `subscribers`; both imports, the repository one under the `persistInvoiceRepo` alias | each function by name, then the import block |
| `restapi/v1/router.go` | `invoice.NewRoutes(api, b, l)` and the import | `NewRoutes` by name |
| `grpc/v1/router.go` | `v1.RegisterInvoiceServiceServer(app, invoice.New(b, l))` and the import | `NewRoutes` by name |
| `graphql/resolver.go` | the `invoiceQuery` and `invoiceMutation` aliases, the embeds in `query` and `mutation`, and both in `newResolver` | each declaration by name |
| `graphql/gqlerr/gqlerr.go` | the two sentinels' entries in `Map` | the `[]mapping` literal in `Map`, ahead of the shared entries |
| `graphql/gqlgen.yml` | the `Invoice` model binding | the `models` map, as a YAML node |

It does not touch `docs/` or `web/`. It prints the docs checklist from CLAUDE.md instead: a
section in `docs/api.md`, the `What you get` list in `README.md`, and the aggregate and projector
tables in `docs/cqrs.md`.

## What `make new-usecase domain=task name=archive-task kind=command` writes

`kind` is `command` or `query`, like the .NET `--usecase-type`.

| Layer | Created or edited |
| --- | --- |
| use case | creates `usecase/task/command/archive_task.go` and its test; edits `register.go` (one `cqrs.HandleCommand` line, or `cqrs.HandleQuery`, in `Register`) |
| REST | edits `restapi/v1/task/task.go` (appends the op and its input and output structs), `controller.go` (registers it in `NewRoutes`), `fake_test.go` (a fake handler for the message in `buses`), and `task_test.go` (appends a test) |
| gRPC | edits `docs/proto/v1/task.proto` (an rpc in the service, plus its two messages), `grpc/v1/task/task.go` (appends the method), and `task_test.go` (its fake and a test) |
| GraphQL | edits `schema/task.graphqls` (a field on `extend type Mutation`, or `Query` for a query), `graphql/task/mutation.go` or `query.go` (appends the resolver), `fake_test.go`, and the test |

The defaults are POST `/v1/tasks/{id}/archive` for a command and GET `/v1/tasks/count` for a query;
`method=` and `path=` override them.

A command answers `cqrs.Written`, like every command. A query answers a result struct of its own,
declared in its file (`query.CountTasksResult`), empty until you add its fields. The command
records nothing yet: its event and the aggregate's behavior method are yours to write, and the
printed checklist names the files.

This command edits files you have already changed by hand, so its edits are the riskiest, and the
safety rules below apply to it in full.

## Safety rules

Everything is planned in memory first: the rendered files and every edit, as old and new bytes.
**Nothing is written until every rule passes for every file.** On the first failure, the generator
prints the rule, the file and the reason, then exits non-zero having written nothing.

| Rule | Check |
| --- | --- |
| R1 name | Matches the pattern. Not a Go keyword, a predeclared name, or a package the generated files import (`command`, `query`, `entity`, `repo`, `cqrs`, `v1`, …). Not an existing domain: no use-case or read-model folder, controller folder on any transport, entity file, proto service, GraphQL type or table with that name. A controller folder can be a package the domains share, such as REST's `response` or GraphQL's `paging`, which the domain's controllers would otherwise join. No identifier the plan adds at a package's top level, in a created file or by an edit, is declared in that package already: `record` would declare `entity.Record`, which exists. |
| R2 targets exist or don't | No file it would create exists. Every file it would edit exists. For `new-usecase`, the domain exists and the message name is free. |
| R3 clean | No file it would edit has uncommitted changes (`git status --porcelain -- <files>`), and neither do the paths the regenerators write (`docs/proto/v1/`, `docs/openapi/v1.json`, `internal/repo/repomock/`, `internal/repo/persistent/queries/`, `internal/controller/graphql/generated/` and `model/`, `web/src/api/schema.gen.ts`, `web/src/graphql/`), so the undo is exact. |
| R4 one anchor | Every edit finds its place exactly once, by parsing the file: `go/ast` for Go, `gqlparser` for `.graphqls`, `protocompile` for `.proto`, `yaml.v3` nodes for `gqlgen.yml`. Zero or two matches fail. An import whose name another import in the file binds fails too, as a domain called `config` would in `buses.go`. No markers, no line numbers, no regular expressions on source. |
| R5 still valid | Every edited and every created file parses after the change. Edited and created Go files go through `go/format`: gofmt aligns columns by the longest name in them, so a created file's alignment depends on the domain's name. A template is written as its file for `widget`, and the golden test fails one whose output for `widget` `go/format` would change: its template is wrong. |
| R6 all or nothing on disk | Each file is written to a temporary file and renamed into place. If any write fails, every file already written is restored and every created file is removed. |

Afterwards it prints the undo command. Because R3 held, the undo restores exactly the previous
state:

```
git checkout -- <edited files> <regenerated paths> && git clean -fd -- <created files and folders> <regenerated paths>
```

## Project structure

```
tools/gen/                       its own module: github.com/m7medvision/go-clean-template/tools/gen
  go.mod                         stdlib, gqlparser, protocompile, go.yaml.in/yaml/v3, testify; nothing from the app
  main.go                        new-domain, new-usecase
  names/                         name forms and plurals
  plan/                          builds the in-memory plan: files to create, edits to apply
  rules/                         R1–R6, one file each
  edit/                          the anchor finders and inserters, one file per file kind
  templates/domain/…             .tmpl files laid out as the repo paths they render to
  templates/usecase/…
  testdata/golden/               expected output for name=widget
```

A folder with its own `go.mod` is outside the app module: `go build ./...`, `make test`, the
binaries and the Docker image never include it. `make gen-check` and `make gen-e2e` cover it, and
`make pre-commit` runs `make gen-check`.

## Code style

Templates are the real file with placeholders, one per output file:

```gotemplate
// Package command holds the {{.Name.Words}} domain's commands, one file each, with their handlers.
package command

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/cqrs"
)

// Create{{.Name.Go}} creates a {{.Name.Words}} owned by UserID.
type Create{{.Name.Go}} struct {
	UserID string
}

// CommandName -.
func (Create{{.Name.Go}}) CommandName() string { return "Create{{.Name.Go}}" }

// ActorID -.
func (c Create{{.Name.Go}}) ActorID() string { return c.UserID }

// Create{{.Name.Go}}Handler handles Create{{.Name.Go}}.
type Create{{.Name.Go}}Handler struct {
	Store repo.EventStore
}

// Handle -.
func (Create{{.Name.Go}}Handler) Handle(_ context.Context, _ Create{{.Name.Go}}) (cqrs.Written, error) {
	return cqrs.Written{}, fmt.Errorf("Create{{.Name.Go}}: %w", entity.ErrNotImplemented)
}
```

`register.go` registers it as `cqrs.HandleCommand(cmds, command.Create{{.Name.Go}}Handler{Store: d.Store})`.
The stub's parameters are `_`, so revive's `unused-parameter` passes; you name them when you write
the body.

`entity.ErrNotImplemented` is added to `internal/entity/errors.go` in this PR. Like
`ErrReadModelBehind` and `ErrConcurrencyConflict`, any command or query can produce it, so it is
mapped once per transport, beside them, and never per domain:
- REST: `apierr.Shared` returns `huma.Error501NotImplemented`.
- gRPC: `rpcerr.Shared` returns `codes.Unimplemented`.
- GraphQL: `gqlerr.Map` gets a `NOT_IMPLEMENTED` code among its shared entries.

Each of the three gets a test case in its package's existing test.

The generator's own Go follows the repo's linters and conventions.

## Testing strategy

| Level | What |
| --- | --- |
| Unit | name forms and plurals; each rule passing and failing on fixture files, including two-anchor and zero-anchor files; each inserter |
| Golden | renders `name=widget` and `domain=widget name=archive-widget` and compares with `testdata/golden/`; `UPDATE_GOLDEN=1` rewrites, as `mailtmpl` does. The same run checks that every template renders `go/format`-clean for `widget`, that every template is used by a layer, and that `widget` planned against the repository's real shared files passes every rule |
| All or nothing | a fixture where the last rule fails leaves the tree byte-identical; a write failure injected half-way restores everything |
| End to end (CI) | `make gen-e2e`: `git worktree add` a throwaway copy, run `make new-domain name=widget` and `make new-usecase domain=widget name=archive-widget kind=command`, then `make linter-golangci test` there. It needs Docker for `make sqlc`, as CI already has. It runs when `tools/gen/**` or any path a template mirrors changes. |

## Boundaries

- **Always:**
  - check every rule before writing;
  - keep the generator in its own module;
  - render from `.tmpl` files;
  - keep the golden files and the e2e job green.
- **Ask first:**
  - generating anything under `web/` or `docs/`;
  - taking field definitions;
  - a third command.
- **Never:**
  - markers in the app's code;
  - editing a file with uncommitted changes;
  - writing part of a plan;
  - importing the app module from `tools/gen`;
  - Claude attribution.

## Success criteria

1. On a clean checkout, `make new-domain name=invoice` creates the files above and edits the
   shared files above. After it, `make pre-commit` passes and the integration tests pass with
   `invoice_test.go`.
2. Every invoice operation answers "not implemented" on REST, gRPC and GraphQL.
3. `make new-usecase domain=task name=archive-task kind=command` adds the operation on all three
   transports, and the same gates pass.
4. A broken rule leaves the tree byte-identical. A test proves this for each of R1 to R6.
5. Running either command twice fails R1 or R2 the second time and writes nothing.
6. `make gen-e2e` passes in CI, and fails when a template no longer matches the code. To prove it,
   the PR history shows one deliberately broken template.
7. `go list -deps ./cmd/app ./cmd/worker` shows nothing from `tools/gen`.

## Known limitations

- **Two untouched generated domains are clones of each other.** Until one of them has fields and
  bodies of its own, `dupl` finds their files in a shared package, such as `internal/entity`, to be
  structural copies. Write one domain before generating the next. A file-level `//nolint:dupl` in
  the templates can't solve this: while only one domain exists the directive suppresses nothing,
  and `nolintlint` rejects it.
- **The read model relies on how sqlc names a table's row struct**: after the singular of the
  table, which holds for the plurals the generator forms. An unusual `plural=` override can break
  that. When it does, the build at the end of `make new-domain` fails and prints the undo.

## Decided

1. **No third command.** `make new-event-handler` is left out of PR 2. Add it if you find yourself
   writing event handlers by hand.
2. **Generated domains are user-owned.** They have a `user_id`, and every handler gets the caller,
   as `task`'s do. Removing ownership is deleting lines; adding it is writing them.
3. **A new query's GraphQL result type** starts with one field, `todo: Boolean`, whose description
   says to replace it; the resolver leaves it null. A GraphQL object type can't be empty, unlike
   the Go result struct.
