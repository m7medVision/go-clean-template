# Scaffold CLI: generate a project with only the features you pick

**Status:** built on branch `feat/scaffold-cli`; see [docs/scaffold.md](../scaffold.md) for how it works now.

## Problem Statement

How might we let us start a new service from this template by picking only the features it needs
(gRPC, GraphQL, frontend, passkeys, queue + worker, email, welcome email, SSO) in an interactive TUI, and get a
project where the dropped features are physically gone, and `make pre-commit` and `make dev` pass
on the first try?

Audience: us and our team. Not a public OSS product yet.

## Recommended Direction

**A manifest-driven stripper that runs inside the template repo.** The template stays the full,
compiling superset. `make dev` in the repo root keeps running every feature, unchanged. The CLI
copies the working tree into `scaffold/out/<name>` and removes what was not picked. Only that copy
is trimmed; the root repo is never touched.

- **`scaffold/features.yaml` is the single source of truth.** For each feature it lists the paths
  it owns, its marker ID, its compose services, Makefile targets and env vars, and its `requires` /
  `conflicts`. It also defines the presets. The CLI reads this file and checks the dependency
  rules; nothing about a specific feature is hard-coded in Go.
- **Shared files use comment markers**: `// scaffold:<feature>:begin` / `:end` in Go,
  `# scaffold:<feature>:begin` / `:end` in the Makefile, YAML and `.env.example`, and
  `<!-- scaffold:<feature>:begin -->` in Markdown (README, `docs/*.md`, CLAUDE.md). Markers do
  nothing to the template build. The CLI deletes the blocks of dropped features and removes the
  marker lines of kept ones.
- **Generated code is regenerated, never stripped.** After stripping, the CLI runs `make sqlc
  graphql proto-v1 mock openapi web-api web-graphql` (only the targets the kept features need), so
  generated files always come from what remains.
- **The scaffolder removes itself**: `cmd/create`, `internal/scaffold`, `scaffold/` and the matrix
  CI job never reach a generated project. The module path is renamed to the one the user entered.

### The TUI

Built with charmbracelet `huh`. `make scaffold` with no arguments opens it:

```
┌ go-clean-template ─────────────────────────────────────┐
│ Project name      › billing-service                    │
│ Module path       › github.com/acme/billing-service    │
│                                                        │
│ Start from a preset                                    │
│   ○ API only (REST)                                    │
│   ● Fullstack (REST + GraphQL + web)                   │
│   ○ Everything                                         │
│   ○ Custom                                             │
│                                                        │
│ Features                    (space: toggle, ↵: next)   │
│   [x] REST            always on                        │
│   [x] GraphQL                                          │
│   [ ] gRPC                                             │
│   [x] Frontend (web/)   needs GraphQL                  │
│   [x] Passkeys                                         │
│   [x] Queue + worker                                   │
│   [x] Email (SMTP)      needs Queue                    │
│   [ ] Welcome email     needs Email                    │
│   [ ] SSO (OIDC)        needs Frontend                 │
│                                                        │
│ ⚠ Unchecking GraphQL will also remove Frontend         │
│                                                        │
│ Output: scaffold/out/billing-service                   │
│ Files kept: 214   removed: 87                          │
│ [ Generate ]   [ Back ]                                │
└────────────────────────────────────────────────────────┘
```

- A preset fills in the checkboxes; changing any box switches the preset to Custom.
- Dependency rules apply as you go: a box whose requirement is missing is greyed out with a
  "needs X" hint, unchecking a feature others depend on warns and offers to drop them too, and an
  invalid combination cannot reach Generate.
- A summary screen (features, output folder, files kept and removed) comes before anything is
  written. After generating, it offers to run `make pre-commit` on the result.
- Flags skip the TUI: `make scaffold PRESET=api` or `make scaffold WITH=grpc,web WITHOUT=email`.
  The CI matrix and `scaffold-check` use this mode.

### Layout in the repo

```
cmd/create/main.go            # entry point: TUI (huh) or flags
internal/scaffold/            # manifest, dependency check, strip, rename, regenerate
scaffold/features.yaml        # features, markers, requires/conflicts, presets
scaffold/out/                 # generated projects; gitignored and `ignore`d in go.mod
```

`scaffold/out` must be in both `.gitignore` and the `ignore` block of `go.mod`. Otherwise
`go build ./...` and golangci-lint in the template also see the generated project's packages.

### Makefile targets

| Target | What it does |
| --- | --- |
| `make scaffold` | Open the TUI |
| `make scaffold PRESET=api` | Generate `scaffold/out/api` without the TUI |
| `make scaffold WITH=grpc,web WITHOUT=email` | Free-toggle generation; rejects invalid combinations |
| `make scaffold-check PRESET=api` | Generate, then run `make pre-commit` inside the output |
| `make scaffold-dev PRESET=api` | Generate, then `make dev` inside the output |
| `make scaffold-matrix` | `scaffold-check` for every preset (CI runs the same thing) |
| `make scaffold-clean` | `rm -rf scaffold/out` |

Day-to-day loop: build features in the root (`make dev`, all features), then
`make scaffold-check PRESET=<x>` to prove a trimmed project still passes, then `make scaffold-dev
PRESET=<x>` to see it run.

### Features and rules (v1)

| Feature | Requires | Notes |
| --- | --- | --- |
| REST | — | Always on (auth and users live here) |
| GraphQL | — | |
| gRPC | — | |
| Frontend (`web/`) | GraphQL | Tables read through GraphQL |
| Passkeys | — | Its web screens are kept only when Frontend is also on |
| Queue + worker | — | `queue_messages` table, `pkg/worker`, `cmd/worker`, the compose `worker` service, and the example handler (`controller/worker/v1/example.go`) so the worker has something to run without email |
| Email (SMTP) | Queue + worker | `pkg/smtp`, `internal/repo/mailtmpl` (the renderer and its layout, no emails), the `mail` use case and worker handler, `entity/email.go` with no email types, `MAIL_*`/`SMTP_*` env vars, Mailpit. Plumbing ready for your own emails |
| Welcome email | Email | `entity.WelcomeEmail`, its `welcome.{html,txt}.tmpl` and golden files, its `DecodeEmail` case, the `UserNotifier` port and its call in the user use case, and `integration-test/mail_test.go` |
| SSO (OIDC) | Frontend | The provider redirects back to the web route `/sso/callback` |

Presets: `api` (REST), `api-grpc` (REST + gRPC), `fullstack` (REST + GraphQL + web + passkeys + queue + email + welcome email),
`everything`.

## Prerequisite refactor (PR 1, before any CLI code)

The features are currently wired into each other's code, and stripping them would spread markers
over dozens of files. First:

- **Wiring per feature.** Split `initServers` in `internal/app/app.go`: `internal/app/grpc.go` and
  `internal/app/graphql.go` own their transport wiring, as `internal/app/worker.go` already does for
  the worker.
- **Email behind a port.** `internal/usecase/user/user.go` enqueues the welcome email directly.
  Move that behind a `repo.UserNotifier` port with a queue-backed implementation. Dropping
  Welcome email removes the field, the call and their tests through markers; there is no no-op
  type, because it would be dead code in the template.
- **Email types removable one at a time.** `entity.DecodeEmail` switches over every email type,
  and the `mailtmpl` tests list every template. Each email's case, fixture and golden files must
  sit in their own marker block (or own file), so removing the welcome email leaves a renderer
  that still compiles and passes `exhaustive` with zero emails.
- **Compose names.** `docker-compose.yml` hard-codes `container_name: db`, `mailpit`, `jaeger`, so
  a generated project's `make dev` collides with the root stack. Drop `container_name`, set a
  compose `name:` per project, and give generated projects their own default ports
  (`PG_PORT`, `HTTP_PORT`, `GRPC_PORT`).

## Testing

1. **Unit tests** in `internal/scaffold` (run by `make test`): the dependency check, preset
   expansion, marker stripping on fixture files, module rename, unbalanced or unknown markers
   rejected.
2. **Identity test:** generating with every feature on equals the template minus the scaffolder
   and marker lines.
3. **Unclaimed-file test:** every file in the repo is owned by a feature, marked core, or listed as
   shared. A new gRPC file nobody claimed fails the test instead of surviving into a gRPC-free
   project.
4. **Matrix:** `make scaffold-matrix`, every preset through `make pre-commit`. Runs in CI and on
   demand, not inside `make test` (too slow).

## Key Assumptions to Validate

- [ ] Markers stay manageable after the refactor: spike stripping gRPC alone and count marker
      blocks. More than ~15 means more refactoring is needed first.
- [ ] Regenerating instead of stripping covers every generated file: run the gRPC-free spike
      through `make pre-commit` with the generated files deleted up front.
- [ ] `huh` can do live dependency feedback (greying out and cascading unchecks) on a multi-select:
      prototype the feature screen before building the rest.
- [ ] Presets cover real needs: if the last few services we started would each have needed
      Custom, the matrix must test more combinations than the presets.

## MVP Scope

**In:**

- the prerequisite refactor
- `features.yaml`, `internal/scaffold`, and `cmd/create` with the TUI and flags
- the features gRPC, GraphQL, Frontend, Passkeys, SSO, Queue + worker, Email, Welcome email, with the four presets
- markers in code, the Makefile, compose, `.env.example`, README, `docs/*.md` and CLAUDE.md
- the `scaffold*` Makefile targets and the CI matrix

**Out:** everything in Not Doing.

## Not Doing (and Why)

- **A `text/template` / cookiecutter-style generator.** It stops the template from compiling, so
  we would lose `make pre-commit`, lint and `make dev` on the source of truth.
- **Disabling features with flags or build tags.** It leaves dead code behind, and it contradicts
  the rule of no on/off toggles.
- **Adding a feature to an existing generated project.** It needs merge logic against code the
  user has since changed. Regenerate and copy across instead.
- **Upgrading generated projects from newer template versions.** Same problem; revisit only if we
  keep many generated services alive.
- **Email without the queue** (sending SMTP inline from the request). It would be a second
  delivery path with different failure behaviour, so Email always requires Queue + worker.
- **Testing all 2^n combinations in CI.** Presets are tested; Custom combinations are checked by
  the dependency rules and `make scaffold-check` on demand.
- **Publishing for outside users** (`go run …@latest`, docs for strangers). The audience is us for
  now; `--from <git-url>` can come later.
- **Removing domains (task, exchange).** They are the reference code the template teaches with.
  Removing them is a separate question.

## Open Questions

- Is REST really always on, or should a gRPC-only service be possible? Auth (login, passkeys)
  lives on REST today.
  <human-answer> no
- Where do the passkey web screens live when Passkeys is on but Frontend is off? (Assumed: they
  are part of Frontend and dropped.)
- Should `scaffold/out/<name>` ever be written outside the repo (`--out ../billing-service`), and
  does it then run `git init`?
