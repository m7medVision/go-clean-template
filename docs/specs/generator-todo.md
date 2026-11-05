# Tasks: domain and use-case generator

Plan: [generator-plan.md](generator-plan.md) · Spec: [generator.md](generator.md)

**Gates used below:**
- **G1:** `make format`, `make linter-golangci`, `make test` (the app module).
- **GG:** `make gen-check` (the `tools/gen` module).
- **GE:** GG, then `make gen-e2e`.
- **G2:** G1, then `make compose-up-integration-test` with `make dev` stopped.

A **scratch copy** is a throwaway `git worktree` of the branch; nothing is generated into the
working checkout. In task names and checks, `widget` and `invoice` are example domain names.

**Sizes:** S = 1–2 files, M = 3–5, L = 6–8. Where a task lists more than 5 files, they are
templates or tests of the same shape.

---

## Phase 1: Foundation

### 1. `entity.ErrNotImplemented`, mapped once per transport
The sentinel every stub returns, mapped beside `ErrReadModelBehind` and `ErrConcurrencyConflict`.
- [x] `apierr.Shared` maps it to `huma.Error501NotImplemented`.
- [x] `rpcerr.Shared` maps it to `codes.Unimplemented`.
- [x] `gqlerr.Map` maps it to a new `NOT_IMPLEMENTED` code, among the shared entries.
- [x] Each mapping has a case in its package's existing test, written first.

**Verify:** G1. **Depends on:** none. **Files:** `internal/entity/errors.go`, `restapi/v1/apierr/apierr.go` + test, `grpc/v1/rpcerr/rpcerr.go` + test, `graphql/gqlerr/gqlerr.go` + test. **Size:** M (seven small edits)

### 2. The `tools/gen` module, its CLI and the make targets
`tools/gen/go.mod` (`github.com/m7medvision/go-clean-template/tools/gen`) and `main.go`. `main.go`
parses `new-domain` and `new-usecase` with all their flags, plus `--root`. Each command builds an
empty plan for now.
- [x] `make new-domain name=x dry-run=1` exits 0 and prints an empty plan. Unknown or missing
      flags exit 2 with the usage.
- [x] `make gen-check` runs, in `tools/gen`, the root `.golangci.yml`, a `gofumpt`/`gci` check,
      `go test -race ./...` and `go mod tidy -diff`, and passes.
- [x] `go list -deps ./cmd/app ./cmd/worker` names nothing under `tools/gen`.
- [x] The root `make format`, `make linter-golangci` and `make test` behave as before.

**Verify:** GG, G1. **Depends on:** none. **Files:** `tools/gen/go.mod`, `main.go`, `main_test.go`, `Makefile`. **Size:** M

### 3. Name forms and plurals
`names/`: every form in the spec's table, from a kebab-case singular.
- [x] `line-item` gives `LineItem`, `lineitem`, `line_item.go`, `line_items`, `/v1/line-items` and
      `lineitem-<uuid>`.
- [x] Plurals follow `-s`, `-es` and `-y` → `-ies`, and `plural=` overrides them.
- [x] A bad pattern, a Go keyword or a predeclared name is an error naming the rule (R1).

**Verify:** GG. **Depends on:** 2. **Files:** `tools/gen/names/names.go`, `names_test.go`. **Size:** S

## Phase 2: Editors

Every editor returns insertions (path, offset, text). Each one must find its place exactly once:
zero or two matches is an error that names the anchor. An editor writes nothing.

### 4. Go: declarations, blocks and imports
`edit/goedit.go`: find a function, type or method by name. Find a `var ( … )` or `type ( … )`
block's closing paren. Add an import, with or without an alias, in sorted position within the
right group.
- [x] Fixtures: an insertion before the closing paren of `errors.go`'s `var` block and of
      `contracts.go`'s `type` block. An aliased import lands in sorted position. Each result is
      `go/format`-clean.
- [x] A fixture with two matching blocks, and one with none, each fails with the anchor's name.
- [x] Comments and blank lines around each insertion are unchanged.

**Verify:** GG. **Depends on:** 2. **Files:** `tools/gen/edit/goedit.go`, `goedit_test.go`, `testdata/go/*`. **Size:** M

### 5. Go: call arguments, literal elements, statements and struct fields
The remaining Go insertions the spec lists:
- an argument in a call, as in `slices.Concat` in `EventTypes`;
- an element after the last element that matches, as in `subscribers` after the last
  `NewProjector`;
- an element before the first element that matches, as in `gqlerr.Map` before the shared entries;
- a statement after the last statement that matches, as in the `Register` lines and the
  `NewRoutes` calls;
- a field or embed in a named struct, as in the `query` and `mutation` structs in `resolver.go`;
- a key in a composite literal inside a function, as in `newResolver`.
- [x] Each kind has a fixture modelled on its real file, passing and failing on zero and on two
      matches.

**Verify:** GG. **Depends on:** 4. **Files:** `tools/gen/edit/goedit.go`, `goedit_test.go`, `testdata/go/*`. **Size:** M

### 6. Proto editor
`edit/proto.go` with `protocompile`'s parser: an `rpc` inside `service <X>Service { … }` before its
closing brace, and messages at the end of the file.
- [x] A fixture modelled on `task.proto` gets the rpc and both messages, and still parses.
- [x] A missing service and a duplicate service each fail.

**Verify:** GG. **Depends on:** 2. **Files:** `tools/gen/edit/proto.go`, `proto_test.go`, `testdata/proto/*`. **Size:** S

### 7. GraphQL schema editor
`edit/graphql.go` with `gqlparser`: a field on the file's `Query` or `Mutation`, whether it is
written `type` or `extend type`, before its closing brace. The brace is found with `gqlparser`'s
lexer from the definition's start, never with a regular expression.
- [x] Fixtures modelled on `task.graphqls` (`type Mutation`) and `exchange.graphqls`
      (`extend type Query`) pass, and still parse.
- [x] A file without the root type fails, naming it.
- [x] Descriptions and comments inside the type do not confuse the brace search.

**Verify:** GG. **Depends on:** 2. **Files:** `tools/gen/edit/graphql.go`, `graphql_test.go`, `testdata/graphql/*`. **Size:** S

### 8. `gqlgen.yml` editor
`edit/yaml.go` with `yaml.v3` nodes: an entry in the `models` map after its last entry, at the
map's indentation.
- [x] A fixture modelled on `gqlgen.yml` gets `Widget: {model: …/entity.Widget}`. The result parses,
      and every other line is byte-identical.
- [x] A file without `models`, or with the key already present, fails.

**Verify:** GG. **Depends on:** 2. **Files:** `tools/gen/edit/yaml.go`, `yaml_test.go`, `testdata/yaml/*`. **Size:** S

### Checkpoint A: editors
- [x] GG.
- [x] Every editor fails on zero and on two anchors, and leaves comments and layout alone.

## Phase 3: Plan, rules and write

### 9. The plan, the dry run and the undo command
`plan/`: a `Plan` holds the files to create and the insertions to apply. It renders each file's
new bytes in memory. It prints a dry run: the files to create, and each insertion with three
lines of context. It also builds the undo command (decision 5: it covers the regenerators'
outputs too).
- [x] Applying a plan in memory twice gives the same bytes; insertions at one offset keep the
      order they were planned in.
- [x] Dry-run output for a fixture plan matches its golden file.
- [x] The undo command lists the edited files, the regenerated paths and the created files.

**Verify:** GG. **Depends on:** 4. **Files:** `tools/gen/plan/plan.go`, `dryrun.go`, `plan_test.go`, `testdata/plan/*`. **Size:** M

### 10. R1 and R2
`rules/r1_name.go`: the name rules from task 3, plus "not an existing domain": no use-case
folder, entity, proto service, GraphQL type or table with the name. `rules/r2_targets.go`:
no file to create exists, and every file to edit exists. For `new-usecase`, the domain exists,
the message name is free, and every named transport exists for the domain.
- [x] Each check passes and fails on a fixture root, and the failure names the rule, file and
      reason.

**Verify:** GG. **Depends on:** 3, 9. **Files:** `tools/gen/rules/r1_name.go`, `r2_targets.go`, tests, `testdata/root/*`. **Size:** M

### 11. R3, R4 and R5
- **R3:** `git status --porcelain` on every file to edit, and on the regenerators' output paths.
- **R4:** collects every editor's anchor error; one error fails the plan.
- **R5:** every edited and created file still parses. Go goes through `go/format`, and a created
  Go file that `go/format` would change is an error, since its template is wrong.
- [x] Each rule passes and fails on a fixture root. R3 runs against a temporary git repository.
- [x] Rules run in order and report the first failure.

**Verify:** GG. **Depends on:** 9. **Files:** `tools/gen/rules/r3_clean.go`, `r4_anchor.go`, `r5_valid.go`, tests. **Size:** M

### 12. All or nothing on disk (R6)
`plan/write.go`: each file is written to a temporary file, then renamed into place. On any failure,
every edited file is restored and every created file and folder is removed. The undo command is
printed after a successful write.
- [x] A rule failing last leaves a fixture tree byte-identical (compared by hash).
- [x] A write failure injected half-way, through a filesystem interface, restores everything.
- [x] Created folders that did not exist before are removed on rollback.

**Verify:** GG. **Depends on:** 9, 10, 11. **Files:** `tools/gen/plan/write.go`, `write_test.go`. **Size:** S

### Checkpoint B: machinery
- [x] GG.
- [x] A failure of each rule from R1 to R6 leaves the tree byte-identical (success criterion 4).
- [ ] **Your review** of the plan, editor and rule APIs before the templates build on them.

## Phase 4: `new-domain`

From task 14 on, every layer task extends `make gen-e2e`, and that is its verification.

### 13. Entity layer, and `new-domain` wired end to end
Templates for `internal/entity/{{name}}.go`, `_aggregate.go`, `_events.go` and
`_aggregate_test.go`, copied in shape from `task`. They hold the base fields, `Create`, `Update`
and `Delete` on the aggregate, and its stream ID. Edits: `invoiceEvents()` in `EventTypes()`,
and the two sentinels in `errors.go`. `new-domain` runs names → plan → rules → write.
- [x] The golden for `widget` matches, and every created Go file is `go/format`-clean.
- [x] On the real repo, `make new-domain name=invoice dry-run=1` prints four files and two
      insertions and writes nothing.
- [x] In a scratch copy, `make new-domain name=widget` followed by `make test` passes.

**Verify:** GG, then the scratch copy. **Depends on:** 5, 12. **Files:** `tools/gen/templates/domain/internal/entity/*.tmpl` (4), `tools/gen/domain.go`, `testdata/golden/domain/*`. **Size:** L

### 14. The e2e harness
`scripts/gen-e2e.sh` and `make gen-e2e`. The script:
- adds a worktree and carries the working tree's changes into it (decision 4);
- runs `make web-deps`, `make new-domain name=widget`, then `make linter-golangci test` and
  `go vet ./integration-test/...`;
- removes the worktree on exit, pass or fail.

The `make new-domain` recipe runs the regenerators after a real write. If one fails, it prints
the undo.
- [x] `make gen-e2e` passes on the entity layer.
- [x] A deliberately broken entity template makes it fail, and the worktree is still removed
      (checked locally, not committed).

**Verify:** GE. **Depends on:** 13. **Files:** `scripts/gen-e2e.sh`, `Makefile`. **Size:** S

### 15. Schema, queries and read model
Templates for `internal/app/schema/{{plural}}.hcl` (the base columns, no foreign keys),
`queries/{{name}}.sql` (get, list, count, insert, touch, delete) and
`persistent/{{pkg}}/{{name}}.go`. The read model's `NewReadModel` returns
`repo.<Name>ReadModel` wrapped in `newTraced`, with `tracing.go` beside it. Edit: the
`<Name>ReadModel` interface in `contracts.go`.
- [x] The goldens match.
- [x] `make gen-e2e` now also runs `make sqlc` and `make mock` and passes.

**Verify:** GE. **Depends on:** 14. **Files:** 4 templates, `tools/gen/domain.go`, goldens. **Size:** M

### 16. Projector
Templates for `persistent/{{pkg}}/projector.go` (`NewProjector`, `Name`, `Tables`, `Handle`, and
`Project` through `queries.Querier`) and `projector_test.go` against a fake `Querier`. The fake
covers `Created`, `Updated` and `Deleted`, and checks that other domains' events change nothing.
- [x] The generated projector test passes in the e2e copy.

**Verify:** GE. **Depends on:** 15. **Files:** 2 templates, `tools/gen/domain.go`, goldens. **Size:** S

### 17. Commands, queries and wiring
Templates for:
- `usecase/{{pkg}}/register.go` (`Deps{Store, <Plural>}`, `Register`);
- `command/create_`, `update_` and `delete_{{name}}.go`, each with its test against `memstore`,
  and `command_test.go` with what the three share (so `dupl` finds no clones);
- `query/get_{{name}}.go`, `list_{{plural}}.go` and `query_test.go` against `repomock`.

Every handler is a stub returning `entity.ErrNotImplemented`, and each test asserts that, with
the comment the spec asks for. Edits to `internal/app/buses.go`: the `Register` call after the
last one in `registerHandlers`, the projector after the last `NewProjector` in `subscribers`, and
both imports.
- [x] The goldens match, and `make gen-e2e` passes.

**Verify:** GE. **Depends on:** 16. **Files:** 11 templates, `tools/gen/domain.go`, goldens. **Size:** L

### Checkpoint C: a domain without transports
- [x] GE.
- [x] In a scratch copy with `widget` generated, the app starts, and the worker runs the `widget`
      projector: `projection_lag_events{projector="widget"}` appears on `/metrics`.

### 18. REST
Templates for `restapi/v1/{{pkg}}/`:
- `controller.go` (`NewRoutes`, `problem`, `readBack`);
- `{{name}}.go` (the five ops with their input and output structs, under `/v1/{{plural-path}}`);
- `request.go` (empty create and update DTOs);
- `response.go` (`<Name>List`);
- `fake_test.go`, `{{name}}_test.go` and `main_test.go`.

Edit: the `NewRoutes` call in `restapi/v1/router.go`, and its import.
- [x] The generated tests cover encoding, 401, the domain sentinels, 503, 409 and 501, through
      fake handlers.
- [x] `make gen-e2e` now also runs `make openapi` and `make web-api` and passes.

**Verify:** GE. **Depends on:** 17. **Files:** 7 templates, `tools/gen/domain.go`, goldens. **Size:** L

### 19. gRPC
Templates for `docs/proto/v1/{{name}}.proto` and `grpc/v1/{{pkg}}/`: `controller.go` (`New`, the
error mapping), `{{name}}.go`, `request.go`, `response.go`, `response_test.go`, and
`{{name}}_test.go` over `bufconn` with fakes in the test. Edit: the `Register<Name>ServiceServer`
line in `grpc/v1/router.go`, and its import.
- [x] `make gen-e2e` now also runs `make proto-v1` and passes.

**Verify:** GE. **Depends on:** 17. **Files:** 7 templates, `tools/gen/domain.go`, goldens. **Size:** L

### 20. GraphQL
Templates for `schema/{{name}}.graphqls` (`extend type Query` and `extend type Mutation`;
mutations take only `id`, per decision 7) and `graphql/{{pkg}}/`: `resolver.go`, `query.go`,
`mutation.go`, `fake_test.go`, `query_test.go` and `mutation_test.go`. Edits:
- the aliases, embeds and `newResolver` fields in `graphql/resolver.go`;
- the two sentinels in `gqlerr.Map`;
- the model binding in `gqlgen.yml`.
- [x] `make gen-e2e` now also runs `make graphql` and `make web-graphql` and passes.

**Verify:** GE. **Depends on:** 17. **Files:** 7 templates, `tools/gen/domain.go`, goldens. **Size:** L

### 21. Integration test
Template for `integration-test/{{name}}_test.go`. It registers a user, then checks every
operation on every generated transport: REST 501, gRPC `Unimplemented`, GraphQL `NOT_IMPLEMENTED`.
It uses the existing helpers in `helpers_test.go` and `grpc_helpers_test.go`.
- [x] `go vet ./integration-test/...` passes in the e2e copy.
- [x] Once, by hand: a scratch copy with `widget` generated passes G2, including
      `widget_test.go`.

**Verify:** GE, then G2 in the scratch copy. **Depends on:** 18, 19, 20. **Files:** 1 template, `tools/gen/domain.go`, golden. **Size:** S

### 22. `transports=` for `new-domain`
Only the named transports' files and edits are planned. The integration test covers only those.
- [x] Goldens for `transports=rest` and `transports=grpc,graphql`.
- [x] Once, by hand, in a scratch copy: `make new-domain name=widget transports=rest` passes
      `make linter-golangci test`.

**Verify:** GG, then the scratch copy. **Depends on:** 21. **Files:** `tools/gen/domain.go`, `main.go`, goldens. **Size:** S

### Checkpoint D: `new-domain` complete
- [x] GE.
- [x] Scratch copy: `make new-domain name=invoice`, then `make pre-commit` and G2 pass (success
      criteria 1 and 2).
- [x] The same command run again fails R1 and writes nothing (success criterion 5).
- [x] `go list -deps ./cmd/app ./cmd/worker` names nothing under `tools/gen` (success criterion 7).
- [ ] **Your review** of one generated domain.

## Phase 5: `new-usecase`

### 23. Use-case layer for both kinds
`kind=command` creates `command/{{name}}.go` and its test. `kind=query` creates
`query/{{name}}.go`, with its `<Name>Result` struct, and a test. Both add one line to
`register.go`, after the last `HandleCommand` or `HandleQuery`. The domain is detected from its
use-case folder. `transports` defaults to the ones the domain has (decision 6).
- [x] Goldens for `domain=widget name=archive-widget kind=command` and `name=count-widgets kind=query`.
- [x] A domain that does not exist, a message name already taken and a transport the domain lacks
      each fail with the rule that caught it.

**Verify:** GG. **Depends on:** 22. **Files:** 4 templates, `tools/gen/usecase.go`, `main.go`, goldens. **Size:** M

### 24. REST operation
Appends to the domain's REST folder:
- in `{{domain}}.go`, the op and its input and output structs, at the end of the file;
- in `controller.go`, its registration, after the last `r.<op>(api)` call in `NewRoutes`;
- a test, at the end of `{{domain}}_test.go`.

*Changed while building it:* no `fake_test.go` edit. `task`'s fakes and helpers are named
differently from a generated domain's, so the appended test is self-contained: it builds buses
holding only a fake for the new message and serves them with `resttest` (spec updated).

The defaults are POST `/v1/{{plural}}/{id}/<verb>` for a command and GET `/v1/{{plural}}/<noun>`
for a query; `method=` and `path=` override them.
- [x] Goldens for the inserted text of both kinds.
- [x] A fixture domain without `problem` or `NewRoutes` fails R4 by name.

**Verify:** GG. **Depends on:** 23. **Files:** 2 templates, `tools/gen/usecase.go`, goldens, fixtures. **Size:** M

### 25. gRPC method
In the domain's `.proto`, the rpc goes in the service, and its two messages go at the end of the
file. In `grpc/v1/{{domain}}/{{domain}}.go`, the method goes at the end. In `{{domain}}_test.go`,
a test with its own fake goes at the end.

*Changed while building it:* the service is read from the `Unimplemented<Service>Server` the
domain's `Controller` embeds, and the `.proto` file is the one that declares it, since `user`
serves `AuthService` from `auth.proto`; `controller.go` must declare `fail` (R4). `user` and
`exchange` have no gRPC test file, so a domain without one gets it, created with the test's
imports. The test serves the controller over `bufconn` on buses holding only its fake, and calls
the proto and controller packages by the names the test file imports them under (`ImportName`).
- [x] Goldens for both kinds.

**Verify:** GG. **Depends on:** 23. **Files:** 3 templates, `tools/gen/usecase.go`, goldens. **Size:** S

### 26. GraphQL field
A field on the domain file's `Mutation` or `Query`. The resolver goes at the end of `mutation.go`
or `query.go`, the fake in `fake_test.go`, and a test at the end of the test file. A query's
result type starts with the `todo: Boolean` placeholder (spec, Decided 3).

*Changed while building it:* no `fake_test.go` edit, as in task 24: the test is self-contained,
through `graphqltest` on buses holding only its fake. A schema without the root type fails R4,
naming it, and nothing more is planned for the field, so a missing `mutation.go` (R2) cannot hide
why. The default transports leave GraphQL out for a command on such a domain
(`rules.UsecaseTransports`): exchange has no `Mutation`.
- [x] Goldens for both kinds.
- [x] `exchange` with `kind=command` and an explicit `transports=graphql` fails, naming the
      missing `Mutation`.

**Verify:** GG. **Depends on:** 23. **Files:** 3 templates, `tools/gen/usecase.go`, goldens. **Size:** S

### 27. `new-usecase` in the e2e job
`make gen-e2e` also runs, after `new-domain`:
- `make new-usecase domain=widget name=archive-widget kind=command`;
- `make new-usecase domain=widget name=count-widgets kind=query`;
- `make new-usecase domain=task name=archive-task kind=command`, a hand-written domain.

Each is followed by the regenerators and the gates.

*Added while building it:*
- **R2 on the REST route** (`rules.RouteFree`): a use case whose operation ID, or method and path,
  an operation of the app already has is refused before the write, suggesting `path=`. It used to
  be a known limitation that only `make test` caught.
- **R2 on the record field** (`rules.RecordFieldFree`): a use case that acts on one `user` would
  carry its ID as `UserID`, the caller's field, and failed the build after the write; it now takes
  a path without `{id}`.
- **The printed checklist**: both commands print "Next:" after the write and after a passing dry
  run, from `templates/<set>/next.tmpl`, with a golden for each.
- [x] `make gen-e2e` passes.

**Verify:** GE. **Depends on:** 24, 25, 26. **Files:** `scripts/gen-e2e.sh`. **Size:** S

### Checkpoint E: `new-usecase` complete
- [x] GE.
- [x] Scratch copy: `make new-usecase domain=task name=archive-task kind=command`, then
      `make pre-commit` and G2 pass (success criterion 3). Run again, it fails R2.

## Phase 6: Ship

### 28. CI jobs and `make pre-commit`
`.github/workflows/ci.yml` gets two jobs:
- `runner / gen / check`, running `make gen-check`;
- `runner / gen / e2e`, running `make gen-e2e`, with Docker, `protoc` and `setup-vp` as in the
  existing jobs.

Both run when backend paths change (the `changes` filter) and are listed in `ci-ok`.
`make pre-commit` runs `make gen-check`. `.github/dependabot.yml` gets a `gomod` entry for
`/tools/gen`.

*Added while building it:*
- **No `make bin-deps` in the jobs**: its `go install tool` cannot build golangci-lint (nor air)
  from the root module, where the `gobwas/glob` v1.1.0 that svu needs breaks their dependencies.
  Both jobs take golangci-lint from `golangci-lint-action` with `install-only` (the release go.mod
  pins, as the lint job runs), and the e2e job `go install`s the other tools it calls by package.
- **The "Done" line**: after the regenerators and the build pass, `make new-domain` and
  `make new-usecase` print one line pointing back to the checklist and the undo, which the
  regenerators' output has scrolled away.
- [x] Both jobs pass on the PR.
- [x] A change limited to `web/**` skips both, and `ci-ok` still passes.

**Verify:** CI. **Depends on:** 27. **Files:** `.github/workflows/ci.yml`, `.github/dependabot.yml`, `Makefile`. **Size:** S

Confirmed on GitHub: run 41945297265 on PR #47 passed both jobs (gen / check 1m08s, gen / e2e
6m27s) and ci-ok. A change limited to `web/**` skips them: they share the `backend` filter with
`tests / unit`, which PR #43 (only `web/bun.lock`, `web/package.json`) skipped while ci-ok passed.

### 29. Docs
- [x] `docs/generator.md`: both commands, every flag, the safety rules, the undo, and what you
      write after generating.
- [x] `CLAUDE.md`:
  - rows in the commands table;
  - the two "Adding…" recipes start with the generator;
  - a "Keep the docs current" row: a change to a file a template mirrors updates its template
    under `tools/gen/templates`.
- [x] `README.md`: a `What you get` bullet and a link.
- [x] `docs/api.md`: the 501 / `Unimplemented` / `NOT_IMPLEMENTED` answer beside 503 and 409.
- [x] The spec's status reads "implemented".

**Verify:** the links resolve. **Depends on:** D. **Files:** `docs/generator.md`, `CLAUDE.md`, `README.md`, `docs/api.md`, `docs/specs/generator.md`. **Size:** M

### 30. Proof that the e2e job catches a broken template
Success criterion 6: one commit on the PR breaks a template on purpose, its CI run fails in
`runner / gen / e2e`, and the next commit restores it. This pushes a red commit, so it waits for
your go-ahead.
- [ ] The failing run is linked in the PR description.

**Verify:** CI red, then green. **Depends on:** 28. **Files:** one template. **Size:** XS

### Checkpoint F: complete
- [ ] Every success criterion in the spec.
- [ ] CI green on the PR.
