# Implementation Plan: domain and use-case generator

Spec: [generator.md](generator.md) · Tasks: [generator-todo.md](generator-todo.md) · Module `generator`, PR 2 of 2

## Overview

Build the generator's machinery first: the name forms, the parsers that find each edit's place,
the in-memory plan, the safety rules and the all-or-nothing write. The riskiest parts, the
editors, come first among those. Then the templates go in one layer at a time, from entity out to
the transports. The end-to-end job exists from the first layer, so every template is proven to
build, lint and pass its tests in a real copy of the repo while it is written, not at the end.
`new-usecase` comes after `new-domain` because it reuses all of it. CI, docs and the proof that
the e2e job catches a broken template come last.

## Architecture decisions made while planning

1. **Every edit is a text insertion at an offset found by parsing.** The parsers (`go/ast`,
   `protocompile`, `gqlparser`, `yaml.v3`) only locate the place. The generator never re-prints
   an AST or a YAML document, so comments and layout in the shared files survive. Go files then
   pass through `go/format`. Because every edit is an insertion, the dry-run shows each one as its
   inserted lines with three lines of context, with no diff library.
2. **Template paths mirror repo paths,** with the name forms in the path:
   `templates/domain/internal/entity/{{.Name.Snake}}_aggregate.go.tmpl`. The text each edit
   inserts is a template too, at `templates/domain/edits/<repo path>.<place>.tmpl` (as in
   `edits/internal/entity/event.go.EventTypes.tmpl`), so every byte the generator writes is
   reviewable under `templates/`; only the import paths an edit adds are built in code, from the
   module path in `go.mod`, and a created file writes the app's imports as `{{.Module}}/…`
   (task 15). Created Go files go through `go/format`, since gofmt's alignment depends on the
   name's length; each template is written as its file for `widget`, which the golden test
   checks. Golden files end in `.golden`, so `gofumpt`, `gci` and the Go toolchain never touch
   them.
3. **Golden tests cover the files the generator creates. The edits are unit-tested on fixtures,
   and the e2e job covers them against the real shared files.** A golden copy of an edited
   `buses.go` would break on every unrelated change to that file.
4. **The e2e job runs against the working tree, not `HEAD`.** `make gen-e2e` adds a throwaway
   worktree and applies the tracked changes and the untracked files under `tools/gen` to it. It
   commits them inside the worktree, so R3 holds. Then it generates there and runs the gates.
   Uncommitted template work is tested before it is committed.
5. **R3 also covers the regenerators' outputs** (spec updated). `make new-domain` runs
   `make proto-v1 graphql sqlc mock openapi web-api web-graphql`, which rewrites tracked files and
   adds new ones (`invoice.pb.go`, `generated/invoice.generated.go`, `queries/invoice.sql.go`). R3
   requires those output paths to be clean too. The printed undo restores them and removes what
   they added, so it stays exact.
6. **`new-usecase` defaults `transports` to the ones the domain already has** (spec updated).
   `user` has no GraphQL and `exchange` has no `Mutation`. An explicit `transports=` naming one
   the domain lacks fails R2.
7. **Empty shapes** (spec updated). Go structs and proto messages may be empty, so the generated
   request DTOs and messages are empty, as the place to add fields. GraphQL forbids empty input
   and object types. Generated mutations therefore take only `id`, and the domain gets no
   `input.go` until it has a field.
8. **The spec's two open questions take the recommended answers** (spec updated): no
   `new-event-handler` command, and generated domains are user-owned.
9. **`tools/gen` follows the app's rules.** `make gen-check` runs, inside `tools/gen`:
   - the root `.golangci.yml`;
   - a `gofumpt` and `gci` check;
   - `go test -race`;
   - `go mod tidy -diff`.
   The root `make format` already formats `tools/` (it is in `GO_DIRS`).
10. **A command is a list of layers** (task 13). `tools/gen/domain.go` runs `names.Parse`,
    `rules.DomainFree`, then each layer in `domainLayers()`, then `execute`. A layer is one file,
    `layer_<name>.go`, holding a `func(*planner)` that adds its created files
    (`pl.create("<glob under templates/domain>")`) and its edits
    (`editFile(pl, path, edit.ParseGo, …)` with text from `pl.text("<repo path>.<place>")`), plus
    one entry in `domainLayers()`; a transport's layer names its transport and runs only when
    `--transports` asks for it. Read and anchor failures go to the planner's `errs`, never
    returned early, so `rules.Check` reports the first rule in order.
11. **Commits, made only when you ask:**
    - the `ErrNotImplemented` mapping (task 1);
    - module and editors (2–8);
    - plan, rules and write (9–12);
    - `new-domain` (13–22);
    - `new-usecase` (23–27);
    - CI and docs (28–29);
    - the broken-template proof and its fix (30).

## Dependency graph

```
ErrNotImplemented (1)       module + make (2) ─ names (3)
        │                          │
        │        Go editors (4, 5) · proto (6) · GraphQL (7) · YAML (8)   ◀── riskiest, first
        │                          │
        │              plan + dry run (9) ─ rules (10, 11) ─ write (12)    ◀── checkpoint B
        │                          │
        └────────────▶ entity layer (13) ─ e2e harness (14)
                                   │
                  read model (15) ─ projector (16) ─ use cases (17)          ◀── checkpoint C
                                   │
                    REST (18) · gRPC (19) · GraphQL (20)
                                   │
              integration test (21) ─ transports= (22)                     ◀── checkpoint D
                                   │
     new-usecase: use case (23) ─ REST (24) · gRPC (25) · GraphQL (26) ─ e2e (27)  ◀── checkpoint E
                                   │
                   CI (28) ─ docs (29) ─ broken-template proof (30)        ◀── checkpoint F
```

## Task list

### Phase 1: Foundation
- [x] 1. `entity.ErrNotImplemented`, mapped once per transport
- [x] 2. The `tools/gen` module, its CLI and the make targets
- [x] 3. Name forms and plurals

### Phase 2: Editors
- [x] 4. Go: declarations, blocks and imports
- [x] 5. Go: call arguments, literal elements, statements and struct fields
- [x] 6. Proto editor
- [x] 7. GraphQL schema editor
- [x] 8. `gqlgen.yml` editor

### Checkpoint A: editors
- [x] `make gen-check`; every editor fails on zero and on two anchors.

### Phase 3: Plan, rules and write
- [x] 9. The plan, the dry run and the undo command
- [x] 10. R1 and R2
- [x] 11. R3, R4 and R5
- [x] 12. All or nothing on disk (R6)

### Checkpoint B: machinery
- [x] `make gen-check`; a failure of each rule from R1 to R6 leaves the tree byte-identical
      (success criterion 4).
- [ ] **Your review** of the plan, editor and rule APIs before the templates build on them.

### Phase 4: `new-domain`
- [x] 13. Entity layer, and `new-domain` wired end to end
- [x] 14. The e2e harness
- [x] 15. Schema, queries and read model
- [x] 16. Projector
- [x] 17. Commands, queries and wiring

### Checkpoint C: a domain without transports
- [x] `make gen-e2e`; in a scratch copy the worker runs the generated projector.

### Phase 4 continued
- [x] 18. REST
- [x] 19. gRPC
- [x] 20. GraphQL
- [ ] 21. Integration test
- [ ] 22. `transports=` for `new-domain`

### Checkpoint D: `new-domain` complete
- [ ] Success criteria 1, 2, 5 and 7 on a scratch copy.
- [ ] **Your review** of one generated domain.

### Phase 5: `new-usecase`
- [ ] 23. Use-case layer for both kinds
- [ ] 24. REST operation
- [ ] 25. gRPC method
- [ ] 26. GraphQL field
- [ ] 27. `new-usecase` in the e2e job

### Checkpoint E: `new-usecase` complete
- [ ] Success criterion 3 on `task` in a scratch copy.

### Phase 6: Ship
- [ ] 28. CI jobs and `make pre-commit`
- [ ] 29. Docs
- [ ] 30. Proof that the e2e job catches a broken template

### Checkpoint F: complete
- [ ] Every success criterion in the spec; CI green on the PR.

## Risks and mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| `gqlparser` positions mark where a definition starts, not its closing brace | High | Task 7 finds the brace with `gqlparser`'s lexer from the definition's start, before anything depends on it. If that can't be made exact, the edit fails R4; it never falls back to a regular expression. |
| `dupl` (threshold 100) flags generated handlers as clones of `task`'s | Med | The e2e lint catches it in the layer task that causes it. Fix it by shaping the template; a `//nolint:dupl` with a reason is the last resort. |
| A template drifts in shape from the code it mirrors while still compiling | Med | Task 29 adds a CLAUDE.md rule: a change to a file a template mirrors updates the template. The PR checklist asks for it. |
| The e2e job is slow: Docker for `sqlc`, `protoc`, Vite+ and bun | Med | One job with the same caches as `web` and `sqlc-generated`. It runs only when backend paths change. |
| `new-usecase` meets a hand-written domain that lacks an anchor (`problem`, a `Mutation` type) | Med | R4 names the missing anchor and writes nothing. The e2e job runs `new-usecase` on `task`, and a fixture test covers a domain without the anchor. |
| `revive`'s `unused-parameter` or `unparam` flags the stubs | Low | Stub parameters are `_`; the e2e lint proves it. |
| Created files are `gofmt`-clean but not `gofumpt`-clean | Low | `make new-domain` ends with `make format`, and the golden tests fail on a template whose output `go/format` changes. |

## Parallel work

- Tasks 4–8 (the editors) are independent of each other.
- Task 3 is independent of 4–8.
- The layer tasks (15–22) are sequential, because each one extends the same e2e run.
- Task 29 (docs) can start after checkpoint D.

## Open questions

None. A new query's GraphQL result type starts with one field, `todo: Boolean`, whose description
says to replace it (spec updated).
