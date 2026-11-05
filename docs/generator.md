# Generator

Two make targets write the files a new domain or a new use case needs, on every layer: the
package, imports, types, constructors, signatures, wiring, transport handlers and tests. Handler
bodies are stubs that return `entity.ErrNotImplemented`. The result builds and passes lint and its
tests, and from the first run it answers "not implemented" on every transport. Then you write the
bodies.

The generator is its own Go module, [`tools/gen`](../tools/gen). It imports nothing from the app, and
`go build ./...`, `make test`, the binaries and the Docker image never include it. Its design, and the
reasons behind each choice, are in the spec, [specs/generator.md](specs/generator.md).

## Commands

```sh
make new-domain  name=invoice                                # a domain with CRUD stubs
make new-usecase domain=task name=archive-task kind=command  # one command on a domain
make new-usecase domain=task name=count-tasks  kind=query    # one query
make gen-check                                               # the generator's lint and tests
make gen-e2e                                                 # generate in a throwaway worktree, then lint and test it
```

`make new-domain` and `make new-usecase` run `go -C tools/gen run . <command> --root $(CURDIR) …`. If
the generator wrote anything, the target then runs the regenerators its output needs, then a build:

```sh
make proto-v1 graphql sqlc mock openapi web-api web-graphql format
go build ./...
```

So both targets need what those need: `protoc`, Docker (for `make sqlc`), Atlas, the `vp` CLI with
`make web-deps` run once, and the tools `make bin-deps` installs. The first failure stops them, and
the target tells you to run the undo the generator printed ([Undo](#undo)). A dry run writes
nothing, so it runs none of them.

Run directly, the generator takes the same flags with two dashes (`--name invoice`, `--dry-run`), plus
`--root`, the repository root, which defaults to `.`; the targets pass `$(CURDIR)`. It runs no
regenerator itself. It exits 0 when it succeeded, 1 when a rule or the write failed, and 2 for a bad
command line, after printing its usage.

## `make new-domain`

| Flag | Default | Meaning |
| --- | --- | --- |
| `name=` | required | The domain, kebab-case and singular: `invoice`, `line-item`. |
| `plural=` | English rules | The plural, when the rules get it wrong. The rules pluralize the last word: `-s`, `-x`, `-z`, `-ch` and `-sh` take `-es`, a consonant then `-y` becomes `-ies`, anything else takes `-s`. |
| `transports=` | `rest,grpc,graphql` | Comma-separated: the transports the domain gets. |
| `dry-run=1` | off | Print the plan and run every rule; write nothing ([Dry run](#dry-run)). |

From `line-item`, the generator derives `LineItem` (Go types, proto messages, GraphQL types),
`lineitem` (packages and folders), `line_item.go` (files), `line_items` (the table),
`/v1/line-items` (REST paths) and `lineitem-<uuid>` (streams).

### What it writes

A user-owned domain with five use cases: `CreateInvoice`, `UpdateInvoice`, `DeleteInvoice`,
`GetInvoice` and `ListInvoices`. Its fields are `id`, `user_id`, `created_at` and `updated_at`.

**Handler bodies are stubs; everything they sit on is real.** The command and query handlers return
`entity.ErrNotImplemented`. The rest is working code: the aggregate and its events, registration,
the projector and the read model for those four columns, the transport handlers with their DTO
conversion and error mapping, and the tests.

Every file copies the shape of the `task` file at the same path. The files it creates, for
`invoice`:

| Layer | Transport | Created |
| --- | --- | --- |
| entity | all | `internal/entity/invoice.go` (the read shape), `invoice_aggregate.go` (`InvoiceAggregate`, `InvoiceStreamID`, `NewInvoiceAggregate`, `Update`, `Delete`, `Apply`), `invoice_events.go` (`InvoiceCreated`, `InvoiceUpdated`, `InvoiceDeleted`, `invoiceEvents()`), `invoice_aggregate_test.go` |
| read model | all | `internal/app/schema/invoices.hcl` (no foreign keys), `internal/repo/persistent/queries/invoice.sql`, `persistent/invoice/invoice.go` (`NewReadModel`), `tracing.go` |
| projector | all | `persistent/invoice/projector.go`, `projector_test.go` against a fake `Querier` |
| use case | all | `internal/usecase/invoice/register.go`; in `command/`, `create_`, `update_` and `delete_invoice.go`, each with its test against `memstore`, and `command_test.go`; in `query/`, `get_invoice.go`, `list_invoices.go` and `query_test.go` against `repomock` |
| REST | `rest` | `internal/controller/restapi/v1/invoice/`: `controller.go`, `invoice.go`, `request.go`, `response.go`, `fake_test.go`, `invoice_test.go`, `main_test.go` |
| gRPC | `grpc` | `docs/proto/v1/invoice.proto`; `internal/controller/grpc/v1/invoice/`: `controller.go`, `invoice.go`, `request.go`, `response.go`, `response_test.go`, `invoice_test.go` |
| GraphQL | `graphql` | `internal/controller/graphql/schema/invoice.graphqls`; `internal/controller/graphql/invoice/`: `resolver.go`, `query.go`, `mutation.go`, `fake_test.go`, `query_test.go`, `mutation_test.go` |
| integration | all | `integration-test/invoice_test.go`, covering the transports the domain gets |

That is 22 files whatever the transports, and 7 more for each one: 43 with all three. There is no
`importer.go`, since a new domain never had rows without events.

The operations, on each transport:

| Use case | REST | gRPC (`InvoiceService`) | GraphQL |
| --- | --- | --- | --- |
| create | `POST /v1/invoices` | `CreateInvoice` | `mutation createInvoice` |
| list | `GET /v1/invoices` | `ListInvoices` | `query invoices` |
| get | `GET /v1/invoices/{id}` | `GetInvoice` | `query invoice` |
| update | `PUT /v1/invoices/{id}` | `UpdateInvoice` | `mutation updateInvoice` |
| delete | `DELETE /v1/invoices/{id}` | `DeleteInvoice` | `mutation deleteInvoice` |

The create and update request bodies and messages are empty, as the place to add fields. GraphQL has
no empty input type, so its mutations take only `id`, and the domain gets no `input.go` until it has
a field.

The shared files it edits:

| File | Edit | Transport |
| --- | --- | --- |
| `internal/entity/event.go` | `invoiceEvents()` in `EventTypes()` | all |
| `internal/entity/errors.go` | `ErrInvoiceNotFound`, `ErrInvoiceForbidden` | all |
| `internal/repo/contracts.go` | `InvoiceReadModel` and the `InvoiceFilter` its `List` pages with | all |
| `internal/app/buses.go` | `invoice.Register(…)` in `registerHandlers`, the projector in `subscribers`, both imports | all |
| `internal/controller/restapi/v1/router.go` | `invoice.NewRoutes(api, b, l)` and its import | `rest` |
| `internal/controller/grpc/v1/router.go` | `v1.RegisterInvoiceServiceServer(…)` and its import | `grpc` |
| `internal/controller/graphql/resolver.go` | the `invoiceQuery` and `invoiceMutation` aliases, their embeds, and both in `newResolver` | `graphql` |
| `internal/controller/graphql/gqlerr/gqlerr.go` | the two sentinels in `Map` | `graphql` |
| `internal/controller/graphql/gqlgen.yml` | the `Invoice` model binding | `graphql` |

It writes no `docs/*.md` file and nothing under `web/`; the regenerators refresh the generated files
there (`docs/openapi/v1.json`, the web client's types). What it leaves for you is in
[After generating](#after-generating).

### The generated tests

- **Use-case tests** assert that each stub returns `entity.ErrNotImplemented`, and say in a comment
  that the first real implementation replaces them. That test going red is the signal.
- **Transport tests** send through real buses with fake handlers, as every domain's do: REST and
  GraphQL with a `fake_test.go`, gRPC with fakes in the test. They test what is real: decoding,
  encoding and the error mapping, "not implemented" included.
- **Aggregate and projector tests** assert real behaviour.
- **The integration test** registers a user, then checks every operation's answer today: REST `501`,
  gRPC `Unimplemented`, GraphQL `NOT_IMPLEMENTED`. It runs with the rest of the suite, against the
  worker (`make compose-up-integration-test`).

## `make new-usecase`

| Flag | Default | Meaning |
| --- | --- | --- |
| `domain=` | required | The existing domain, as `make new-domain` was given it: `line-item`, not `lineitem`. |
| `name=` | required | The use case, kebab-case: `archive-task`. |
| `kind=` | required | `command` (it changes state) or `query` (it reads it). |
| `plural=` | English rules | The domain's plural, when `make new-domain` was given one. |
| `method=` | `POST` for a command, `GET` for a query | The REST method: `GET`, `POST`, `PUT`, `PATCH` or `DELETE`. |
| `path=` | `/v1/<plural>/{id}/<verb>` for a command, `/v1/<plural>/<verb>` for a query | The REST path. It starts with `/` and holds no parameter but one `{id}`. The verb is the name's first word: `POST /v1/tasks/{id}/archive`, `GET /v1/tasks/count`. |
| `transports=` | the domain's own | Comma-separated. The default leaves out GraphQL when the domain's schema file has no `Mutation` (for a command) or `Query` (for a query) to put the field on, as `exchange` has no `Mutation`. `user` has no GraphQL at all. |
| `dry-run=1` | off | Print the plan and run every rule; write nothing. |

**`{id}` in the path decides whether the use case acts on one record.** Its message then carries the
record's ID (`TaskID`), which the operation fills in from the path. So by default a command has
`UserID` and `TaskID`, and a query only `UserID`. On `user` the record's ID would be `UserID`, the
caller's field, so a use case there needs a path without `{id}`:
`make new-usecase domain=user name=lock-user kind=command path=/v1/user/lock`.

### What it writes

For `archive-task` on `task`, 2 files created and 10 edited:

| Layer | Transport | Created or edited |
| --- | --- | --- |
| use case | all | creates `internal/usecase/task/command/archive_task.go` (or `query/count_tasks.go`) and its test; edits `register.go`, with one `cqrs.HandleCommand` (or `HandleQuery`) line after the last of its kind |
| REST | `rest` | edits `restapi/v1/task/task.go` (the operation and its input and output structs, at the end), `controller.go` (`r.archiveTask(api)` after the last such call in `NewRoutes`) and `task_test.go` (a test, at the end) |
| gRPC | `grpc` | edits `docs/proto/v1/task.proto` (the rpc in the service; its request and response messages at the end), `grpc/v1/task/task.go` (the method) and `task_test.go` (a test). A domain without a gRPC test file, as `user` and `exchange` are, gets one created |
| GraphQL | `graphql` | edits `schema/task.graphqls` (a field on `Mutation` or `Query`; for a query, its `CountTasksResult` type at the end), `graphql/task/mutation.go` or `query.go` (the resolver) and `mutation_test.go` or `query_test.go` (a test, created when missing) |

What each answers:

- **A command** answers `cqrs.Written`, like every command. Its REST operation answers the written ID
  (`{"id": …}`, the `ArchiveTaskResponse` it declares), its gRPC method an `ArchiveTaskResponse`
  holding `id`, and its GraphQL field `ID!`. It records nothing yet: its event and the aggregate's
  behavior method are yours.
- **A query** answers a result struct declared in its file (`query.CountTasksResult`), empty until you
  add its fields. REST answers that struct as it is, gRPC an empty `CountTasksResponse`, and GraphQL a
  `CountTasksResult` type whose one field, `todo: Boolean`, holds the place of yours.

Each transport's test is self-contained: it builds buses holding only a fake for the new message, so
it needs none of the folder's test helpers, and `fake_test.go` is left alone.

A hand-written domain chose its own names, so `new-usecase` reads what it cannot derive from the
domain's files. Each one is an anchor, found exactly once ([R4](#safety-rules)):

- the handler's dependency: the one field of `Deps` in `register.go` of type `repo.EventStore` (a
  command) or `repo.*ReadModel` (a query; `exchange`'s is `History`);
- the REST tag: the one `_*Tag` constant in the domain's REST `controller.go`, which must also
  declare `problem` and `NewRoutes`;
- the gRPC service: the one `Unimplemented<Service>Server` the gRPC `Controller` embeds (`user`'s
  serves `AuthService`), and the `.proto` file in `docs/proto/v1` that declares it (`auth.proto`);
  that `controller.go` must also declare `fail`;
- the names the gRPC test file imports the proto and controller packages under;
- the `Mutation` or `Query` type in `schema/<name>.graphqls`.

## Dry run

`dry-run=1` plans everything in memory and prints it: a summary line, every file it would create, and
every insertion with three lines of context on each side. Then it runs every rule. If one fails it
exits 1; if they all pass it prints the checklist of [After generating](#after-generating). It writes
nothing either way.

```
new-usecase task archive-task (dry run): 2 files to create, 10 to edit

create internal/usecase/task/command/archive_task.go
create internal/usecase/task/command/archive_task_test.go

edit   internal/controller/restapi/v1/task/controller.go
  @@ line 35
    	r.updateTask(api)
    	r.transitionTask(api)
    	r.deleteTask(api)
  + 	r.archiveTask(api)
    }
    …
Every rule passes. Run it again without dry-run=1 to write it.
```

R1, and R2's checks on a use case's domain, message, record and route, run before the plan is
printed: when one of them fails, the dry run prints only the failure.

## Safety rules

Everything is planned in memory first: every rendered file and every edit, as old and new bytes.
**Nothing is written until every rule passes for every file.** On the first failure the generator
prints the rule, the file and the reason, and exits 1 having written nothing:

```
gen: R1 name: "task" is taken: internal/usecase/task exists
```

| Rule | Refuses |
| --- | --- |
| R1 name | A name that does not match `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`. A Go keyword, a predeclared name, or a package the generated files import (`command`, `query`, `entity`, `repo`, `cqrs`, `v1`, …). A word, in the name or its plural, that gqlgen or revive writes as an initialism and the generator would not (`api-key`; `http2`, though `utf8` passes). A letter right after a digit (`mp3player`), which protoc-gen-go capitalizes, and a word of the plural that starts with a digit (`plural=items-2`). For a domain, a name in use: a use-case or read-model folder, a controller folder on any transport (including a shared package, such as REST's `response` or GraphQL's `paging`), an entity file, a proto service, a GraphQL type or a table. Any identifier the plan adds to a package that the package declares already: `record` would add `entity.Record`; `login` would add `LoginResponse`, which `auth.proto` declares; `all-task` would add `Query.allTasks`. |
| R2 targets | A file to create that exists, or a file to edit that does not. For a use case: a domain that does not exist, a transport asked for that the domain lacks, a message of its kind with the name already, and a REST operation ID or method and path the app already registers (parameter names aside), with a hint to give `path=`: `count-tasks` and `count-done-tasks` both default to `GET /v1/tasks/count`. A path with `{id}` on `user`. |
| R3 clean | Uncommitted changes, untracked files included, in any file it would edit or in the paths the regenerators write: `docs/proto/v1/`, `docs/openapi/v1.json`, `internal/repo/repomock/`, `internal/repo/persistent/queries/`, `internal/controller/graphql/generated/` and `model/`, `web/src/api/schema.gen.ts`, `web/src/graphql/`. This is what makes the [undo](#undo) exact. |
| R4 one anchor | An edit whose place it finds zero times or twice. Places are found by parsing (`go/ast` for Go, `gqlparser` for `.graphqls`, `protocompile` for `.proto`, `yaml.v3` nodes for `gqlgen.yml`), never by markers, line numbers or regular expressions. It also refuses an import whose name another import in the file binds, as a domain called `config` would in `buses.go`. |
| R5 still valid | A created or edited file that does not parse afterwards. Every Go file goes through `go/format`, since gofmt aligns columns by the longest name in them; a template whose output for `widget` `go/format` would change fails the golden tests, as a wrong template. |
| R6 all or nothing | A half-written tree. Each file is written to a temporary file and renamed into place; if any write fails, every edited file gets its old bytes back and every created file and folder is removed. |

R1 runs on the name before anything is planned, and on the plan's identifiers once it is made. Then
R2 to R5 run in order, and the first failure is the one reported. R6 is the write itself.

Run either command twice and the second run fails R1 (`new-domain`) or R2 (`new-usecase`), and
writes nothing.

## Undo

After a write the generator prints how many files it created and edited, the undo command, and the
checklist:

```
git checkout -- <edited files> <regenerated paths> && git clean -fd -- <created files and folders> <regenerated paths>
```

It takes the tree back to where it was, the regenerators' output included, so run it if a
regenerator or the build fails, or if you change your mind. It is exact because of R2 and R3: every
file it created was new, and every file it edited and every path the regenerators write had no
uncommitted changes. So `git checkout` gives back exactly what was there, and `git clean` removes only
what this run added. A folder the generator created stands for everything in it.

`make format`, the last regenerator, formats every Go file in the tree. A change of yours elsewhere
that was not formatted comes out formatted, and the undo leaves it that way.

## After generating

The generator prints what is left to write, from
[`templates/domain/next.tmpl`](../tools/gen/templates/domain/next.tmpl) and
[`templates/usecase/next.tmpl`](../tools/gen/templates/usecase/next.tmpl). After a domain:

- **The handlers**, which return `entity.ErrNotImplemented` until you write them:
  `command/create_invoice.go`, `update_invoice.go`, `delete_invoice.go`, `query/get_invoice.go`
  and `list_invoices.go`. Each has a test that says so, which goes red once you have: replace it with
  tests of what the handler does.
- **The docs**: the domain's section in [api.md](api.md), its bullet in `What you get` in the
  README, and its aggregate and projector rows in [cqrs.md](cqrs.md).
- **This domain, before you generate another** ([Known limitations](#known-limitations)).

The fields are yours too. The generated comments say where each one goes: the read shape in
`internal/entity/invoice.go`, the events that set it and the aggregate's `Apply`, the table and its
SQL (then `make sqlc`), the projector, the create and update DTOs on REST and gRPC, the proto
messages and `NewInvoiceResponse`, and the GraphQL type, with a `CreateInvoiceInput` and
`UpdateInvoiceInput` (and an `input.go`) once there is a first field.

After a command:

- **The handler**, `command/archive_task.go`: change the record through `aggregate.Change`, then
  replace its test with tests of what it records.
- **The event it records** in `internal/entity/task_events.go` and in `EventTypes()`, the behavior
  method that records it with its case in `Apply`, and its case in the projector.
- **A row** for it in the domain's table in [api.md](api.md).

After a query:

- **The handler**, `query/count_tasks.go`: read the answer from the read model, then replace its test
  with tests of what it reads.
- **The read-model method** it calls, in `internal/repo/contracts.go`, the read model and its
  `tracing.go`, with its SQL in `queries/<domain>.sql`; then `make sqlc` and `make mock`.
- **The fields of the answer**, in the `Result` struct, the gRPC response message and the GraphQL
  result type, in place of its `todo` field.
- **A row** for it in the domain's table in [api.md](api.md).

[CLAUDE.md](../CLAUDE.md#adding-a-whole-domain) has the full recipes, starting from the generator.
Before pushing, run `make pre-commit` and the integration tests, as for any change.

## Known limitations

- **Two untouched generated domains are clones of each other.** Until one of them has fields and
  bodies of its own, `dupl` finds their files in a shared package, such as `internal/entity`, to be
  copies, and `make linter-golangci` fails. Write one domain before you generate the next. A
  `//nolint:dupl` in the templates cannot help: while only one domain exists it suppresses nothing,
  and `nolintlint` rejects it.
- **Two untouched use cases of one kind on one domain are clones too**, for the same reason: two
  commands' REST operations, REST tests and handler tests look alike to `dupl`. Write the first one's
  body before you generate the second. A command and a query on one domain are not clones.
- **The read model relies on how sqlc names a table's row struct**: after the table's singular, which
  holds for the plurals the generator forms. An unusual `plural=` can break that. The build at the
  end of `make new-domain` then fails, and the undo restores the tree.

## Changing the generator

The code is in [`tools/gen`](../tools/gen):

| Path | Holds |
| --- | --- |
| `main.go` | the two commands and their flags |
| `domain.go`, `layer_<name>.go` | `new-domain`: `domainLayers()` lists the layers, one file each |
| `usecase.go`, `usecase_<name>.go` | `new-usecase`: `usecaseLayers()`, the same way |
| `names/` | the name forms and plurals |
| `edit/` | the editors that find each insertion's place by parsing, one file per kind of file |
| `plan/` | the in-memory plan, the dry run, the undo and the all-or-nothing write (R6) |
| `rules/` | R1 to R5 |
| `templates/domain/`, `templates/usecase/` | every byte the generator writes, as `.tmpl` files laid out like the repository |
| `testdata/golden/` | the expected output for `widget` |

### Templates

The package doc of [`templates/templates.go`](../tools/gen/templates/templates.go) holds the
conventions:

- **A created file** is `<set>/<repo path>.tmpl`, where the set is `domain` or `usecase`. Its path is
  a template too, rendered with the same data:
  `domain/internal/entity/{{.Name.Snake}}.go.tmpl` creates `internal/entity/widget.go`.
- **The text an edit inserts** is `<set>/edits/<repo path>.<place>.tmpl`, such as
  `domain/edits/internal/entity/event.go.EventTypes.tmpl`. Even a one-line insertion has its own file,
  so everything the generator inserts can be reviewed under `templates/`. Only the import paths an
  edit adds are built in code. A created file writes the app's imports as `{{.Module}}/internal/…`.
- **`<set>/next.tmpl`** is the checklist the command prints.
- **A template reads as its file for `widget`**: for a use case, the `widget` domain's
  `archive-widget` (the command) and `count-widgets` (the query). The generator formats the Go files
  it creates, since gofmt's alignment depends on the name's length, but for `widget` formatting must
  change nothing.
- **The data**: `.Module`; `.Name`, the domain, in each form (`.Name.Go`, `.Name.Lower`, `.Name.Pkg`,
  `.Name.Snake`, `.Name.Kebab`, `.Name.Words`, and the same under `.Name.Plural`); `.Transports`, with
  `{{if .Has "rest"}}` to check one; and for a use case `.Usecase`, with its own `.Usecase.Name`, its
  kind, route and the names read from the domain. A field the data lacks fails the render.
- **Three habits** keep templates parsing and reading well. In Go, a brace right before an action is
  written `{ {{- .Name.Go}}`, since `{{{` does not parse. Two braces that open nested composite
  literals are written `{{"{{"}}`. Comments say "the" or "one" before a name, never "a" or "an",
  which would depend on how the name sounds.

To add a file to a layer, add its template where the layer's `pl.create` pattern matches it. To add
an edit, add its text under `edits/` and an `editFile` call in the layer, finding the place with one of
the `edit` package's anchors. A new layer is one `layer_<name>.go` (or `usecase_<name>.go`) and one
entry in `domainLayers()` (or `usecaseLayers()`).

### Goldens and checks

The golden tests render `widget` (on every transport, and on `rest` and on `grpc,graphql`),
`archive-widget` and `count-widgets`, and compare every created file, the use cases' dry runs and the
checklists with `testdata/golden/`. Golden files end in `.golden`, so no formatter or Go tool touches
them. After a template change:

```sh
UPDATE_GOLDEN=1 go -C tools/gen test ./...   # rewrite the goldens, then review their diff
make gen-check                                # lint and format check (the root .golangci.yml), go test -race, go mod tidy -diff
make gen-e2e                                  # generate in a throwaway worktree, then lint and test the result
```

The same tests check that every template renders `go/format`-clean for `widget`, that every template
is used by a layer, and that planning `widget`, and a command and a query on each of `task`, `user`
and `exchange`, against the repository's real files passes every rule. So `make gen-check` fails when
a shared file or a hand-written domain loses an anchor the generator needs. A change to what
`new-domain` creates or edits also changes the counts that `domain_test.go` and `main_test.go` assert
("43 files to create, 9 to edit").

`make gen-e2e` ([`scripts/gen-e2e.sh`](../scripts/gen-e2e.sh)) runs against the working tree as it is
on disk, not `HEAD`, so template work is tested before it is committed. It adds a throwaway git
worktree, copies in the tracked changes and the untracked files under `tools/gen`, and commits them
there so R3 holds. Then it runs `make new-domain name=widget`, and `make new-usecase` for
`archive-widget` and `count-widgets` on `widget` and for `archive-task` on `task`, a hand-written
domain. After each it checks that the regenerators made code of it, and runs
`make linter-golangci test`; after the domain, `go vet ./integration-test/...` too. The worktree is
removed on exit, pass or fail. It needs the same tools as `make new-domain`, and takes a few
minutes.

CI runs both on every backend change: `runner / gen / check` runs `make gen-check`, and
`runner / gen / e2e` runs `make gen-e2e` once it passes. `make pre-commit` runs `make gen-check`.

### When the app changes

**A change to a file a template mirrors updates its template.** Each domain template copies the
`task` file at the same path, and the use-case templates copy `task`'s use cases and the operations
in its transport files. A change to how `task` is written (a new convention in its handlers, a
renamed helper, a different test shape) belongs in the matching template under
`tools/gen/templates` too, with its golden. So does a change to the shape of a shared file the
generator edits: the nine in the table above, and the domain files `new-usecase` edits. Then
`make gen-e2e` must pass. Code that still compiles but no longer looks like `task` is the drift the
e2e job cannot see, so the template is updated in the same change.
