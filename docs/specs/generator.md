# Spec: domain and use-case generator

Module id: `generator` · PR 2 of 2 · Depends on `cqrs` · Status: **draft, awaiting approval**

Paths below follow the `cqrs` spec. They are checked against the merged `cqrs` code before this
module's plan is written.

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

It stops at the first failure and prints the undo command. `transports` defaults to all three,
since transports are independent by design. `dry-run=1` prints every file it would create and the
diff of every edit, and writes nothing.

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
| entity | `invoice.go` (`Invoice`, `InvoiceAggregate`), `invoice_events.go`, `invoice_test.go` |
| use case | `usecase/invoice/register.go`; `command/` and `query/` with one file per message and its stub handler, plus a test each |
| repo | `persistent/invoice/` read model, `projector.go`, `tracing.go`, tests; `queries/invoice.sql` |
| schema | `internal/app/schema/invoices.hcl` (the base columns) |
| REST | `restapi/v1/invoice/` `controller.go`, `invoice.go`, `request.go`, `response.go`, `invoice_test.go`, `main_test.go` |
| gRPC | `docs/proto/v1/invoice.proto`; `grpc/v1/invoice/` `controller.go`, `invoice.go`, `request.go`, `response.go`, `invoice_test.go` |
| GraphQL | `schema/invoice.graphqls`; `graphql/invoice/` `resolver.go`, `query.go`, `mutation.go`, `input.go`, tests |
| integration | `integration-test/invoice_test.go` |

**Generated tests:**
- They assert today's answer on every transport: REST 501, gRPC `Unimplemented`, GraphQL
  `NOT_IMPLEMENTED`.
- They assert real behaviour for the real parts: the aggregate, the projector, the error mapping.
- Each stub test says, in a comment, that the first real implementation should replace it. That
  test going red is the intended signal.

**Shared files it edits:**

| File | Edit | Found by |
| --- | --- | --- |
| `internal/entity/event.go` | the domain's events in `EventTypes()` | the return value's composite literal in that function |
| `internal/repo/contracts.go` | the `InvoiceReadModel` interface | the `type ( … )` block, before its closing paren |
| `internal/app/` (where `cqrs` builds the buses) | `invoice.Register(…)` and the import | the function by name |
| `internal/app/worker.go` | the invoice projector in the subscriber list | the composite literal by name |
| `restapi/v1/router.go`, `grpc/v1/router.go` | `invoice.NewRoutes(…)` and the import | `NewRoutes` by name |
| `graphql/resolver.go` | the alias, the embeds in `query` and `mutation`, the constructor | each declaration by name |
| `graphql/gqlgen.yml` | the `Invoice` model binding | the `models` map, as a YAML node |

It does not touch `docs/` or `web/`. It prints the docs checklist from CLAUDE.md instead.

## What `make new-usecase domain=task name=archive-task kind=command` writes

`kind` is `command` or `query`, like the .NET `--usecase-type`.

| Layer | Created or edited |
| --- | --- |
| use case | creates `usecase/task/command/archive_task.go` and its test; edits `register.go` (one handler line) |
| REST | edits `restapi/v1/task/task.go` (appends the op and its input and output structs), `controller.go` (registers it in `NewRoutes`), and `task_test.go` (appends a test) |
| gRPC | edits `docs/proto/v1/task.proto` (an rpc in the service, plus its two messages), `grpc/v1/task/task.go` (appends the method), and the test |
| GraphQL | edits `schema/task.graphqls` (a field on `extend type Mutation`, or `Query` for a query), `graphql/task/mutation.go` or `query.go` (appends the resolver), and the test |

The defaults are POST `/v1/tasks/{id}/archive` for a command and GET `/v1/tasks/count` for a query;
`method=` and `path=` override them.

This command edits files you have already changed by hand, so its edits are the riskiest, and the
safety rules below apply to it in full.

## Safety rules

Everything is planned in memory first: the rendered files and every edit, as old and new bytes.
**Nothing is written until every rule passes for every file.** On the first failure, the generator
prints the rule, the file and the reason, then exits non-zero having written nothing.

| Rule | Check |
| --- | --- |
| R1 name | Matches the pattern. Not a Go keyword or predeclared name. Not an existing domain: no folder, entity, proto service, GraphQL type or table with that name. |
| R2 targets exist or don't | No file it would create exists. Every file it would edit exists. For `new-usecase`, the domain exists and the message name is free. |
| R3 clean | No file it would edit has uncommitted changes (`git status --porcelain -- <files>`), so the undo is exact. |
| R4 one anchor | Every edit finds its place exactly once, by parsing the file: `go/ast` for Go, `gqlparser` for `.graphqls`, `protocompile` for `.proto`, `yaml.v3` nodes for `gqlgen.yml`. Zero or two matches fail. No markers, no line numbers, no regular expressions on source. |
| R5 still valid | Every edited and every created file parses after the change, and Go files come out of `go/format` unchanged. |
| R6 all or nothing on disk | Each file is written to a temporary file and renamed into place. If any write fails, every file already written is restored and every created file is removed. |

Afterwards it prints the undo command. Because R3 held, the undo restores exactly the previous
state:

```
git checkout -- <edited files> && rm -r <created files and folders>
```

## Project structure

```
tools/gen/                       its own module: github.com/m7medvision/go-clean-template/tools/gen
  go.mod                         stdlib, gqlparser, protocompile, yaml.v3; imports nothing from the app
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
// Package command holds the {{.Name.Lower}} domain's commands, one file each, with their handlers.
package command

// Create{{.Name.Go}} creates a {{.Name.Words}} owned by UserID.
type Create{{.Name.Go}} struct {
	UserID string
}

// CommandName -.
func (Create{{.Name.Go}}) CommandName() string { return "Create{{.Name.Go}}" }

type create{{.Name.Go}}Handler struct {
	store repo.EventStore
}

func (h create{{.Name.Go}}Handler) Handle(ctx context.Context, cmd Create{{.Name.Go}}) (cqrs.Written, error) {
	return cqrs.Written{}, fmt.Errorf("Create{{.Name.Go}}: %w", entity.ErrNotImplemented)
}
```

`entity.ErrNotImplemented` is added to `internal/entity/errors.go` in this PR. Each transport
maps it to its own "not implemented": `huma.Error501NotImplemented`, `codes.Unimplemented`, and
GraphQL `NOT_IMPLEMENTED`.

The generator's own Go follows the repo's linters and conventions.

## Testing strategy

| Level | What |
| --- | --- |
| Unit | name forms and plurals; each rule passing and failing on fixture files, including two-anchor and zero-anchor files; each inserter |
| Golden | renders `name=widget` and `domain=widget name=archive-widget` and compares with `testdata/golden/`; `UPDATE_GOLDEN=1` rewrites, as `mailtmpl` does |
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

## Open questions

1. **A third command, `make new-event-handler domain=user event=UserRegistered name=send-welcome-email`?**
   The CQRS shape has event handlers, and the .NET template has an `EventHandlers` folder, but its
   generator doesn't make them. I'd leave it out of PR 2 and add it if you find yourself writing
   event handlers by hand.
2. **Should generated domains be user-owned by default?** That means a `user_id`, and every handler
   gets the caller as task does. Three of the four data domains are user-owned. Removing
   ownership is deleting lines; adding it is writing them, so I default to owned.
