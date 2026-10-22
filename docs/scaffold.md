# Scaffolding a project

`make scaffold` generates a new service from this template with only the features you pick. The
template itself always keeps every feature: `make dev`, `make pre-commit` and CI run all of it.
The generated project lands in `scaffold/out/<name>`, a git repository of its own that the
template ignores.

## Using it

| Command | What it does |
| --- | --- |
| `make scaffold` | Opens the interactive form: name, module path, preset, feature checkboxes, summary |
| `make scaffold PRESET=api` | Generates `scaffold/out/api` without the form |
| `make scaffold WITH=grpc,web WITHOUT=email` | Adds and removes features (see below) |
| `make scaffold NAME=billing MODULE=github.com/acme/billing PRESET=fullstack` | Names the project and its module |
| `make scaffold-check PRESET=api` | Generates, then runs `make pre-commit` inside the project |
| `make scaffold-dev PRESET=api` | Generates, then runs `make dev` inside it on other ports (app 8090, web 3100, Postgres 5442, Mailpit 1125/8125, Keycloak 8280), in its own tmux session beside the root's `make dev` |
| `make scaffold-matrix` | `scaffold-check` for every preset, with a pass/fail summary |
| `make scaffold-clean` | Removes `scaffold/out` |

Beside the root, a generated project's SSO sign-in cannot complete: the development realm
(`keycloak/realm.json`) only accepts redirects to port 3000. Stop the root's `make dev` and run
the project on the default ports (`make dev` inside it) to try SSO there.

Selection rules: a preset is the starting point; `WITH` adds features and `WITHOUT` removes them.
With no preset, the start is every feature, unless `WITH` is given alone, which starts from the
core (REST). A selection that breaks a requirement is refused and the message names it
(`web needs graphql`); nothing is added for you. The form enforces the same rules: an invalid
selection cannot be submitted.

| Feature | id | Needs |
| --- | --- | --- |
| REST | `rest` | always included |
| GraphQL | `graphql` | |
| gRPC | `grpc` | |
| Frontend (`web/`) | `web` | GraphQL |
| Passkeys | `passkeys` | |
| SSO (OIDC) | `sso` | Frontend |
| Queue + worker | `queue` | |
| Email (SMTP) | `email` | Queue + worker |
| Welcome email | `welcome` | Email |

Presets: `api` (REST), `api-grpc` (REST + gRPC), `fullstack` (REST + GraphQL + Frontend +
Passkeys + Queue + Email + Welcome email), `everything`.

Generating runs these steps:

1. Copy every file git tracks or would track, so uncommitted changes are included.
2. Skip the paths of dropped features and the scaffolder's own.
3. Strip marked blocks.
4. Rename the module.
5. `git init`.
6. Run the manifest's `regen` steps in the project: `make sqlc`, `mock`, `proto-v1`, `graphql`,
   `deps`, `openapi`, the web codegen, `make format` and `golangci-lint fmt`.

`make sqlc` needs Docker, as it does in the template.

## How a feature is described

[`scaffold/features.yaml`](../scaffold/features.yaml) lists every feature:

- `paths`: the files it owns. A project without the feature has none of them. A path owned by
  several features (`web/src/passkey/**` is under both Frontend and Passkeys) is kept only when
  all of them are.
- `requires`: the features it needs.
- `leaks` and `allow`: the words a project without it must not contain, and the lines exempt from
  them.

`self` lists the scaffolder's paths, and `core` lists the paths every project gets.

Code for a feature inside a shared file sits between markers, in the file's own comment syntax:

```go
// scaffold:grpc:begin
newGRPCServer(cfg, uc, jwtManager, l),
// scaffold:grpc:end
```

```make
# scaffold:grpc:begin
proto-v1:
	...
# scaffold:grpc:end
```

In Markdown the markers are HTML comments: `<!-- scaffold:grpc:begin -->`, `<!-- scaffold:grpc:end -->`.

- A marker line holds nothing else.
- A trailing marker (`restapi.WithOIDC(uc.oidc), // scaffold:sso`) drops just its line.
- A block can be tagged with several features (`scaffold:passkeys,web`). It is kept only when all
  of them are.
- `self` blocks are always dropped. They hold the scaffolder's own Makefile targets and ignore
  rules.
- Comment syntax by file type:
  - `//`: Go, TypeScript, `.proto`, `go.mod`
  - `#`: Makefile, YAML, `.env*`, Dockerfile, Caddyfile, shell, HCL, TOML, GraphQL schemas
  - `<!-- -->`: Markdown and HTML
  - `--`: SQL
- JSON has no comments, so a JSON file must belong wholly to one feature or to none.

When stripping, Go files lose any imports that only the removed code used. A blank line left next
to another blank line by a removal goes too.

## Adding an optional feature

1. Put its own files under paths you can list in `features.yaml` (a package, a `config/<name>.go`,
   an `integration-test/<name>_test.go`). Wire it through one call or one option, as gRPC
   (`internal/app/grpc.go`) and passkeys (`restapi.WithPasskeys`) do, so the shared files need
   few markers.
2. Add the feature to `features.yaml` with its `paths`, `requires` and `leaks`, and to the presets
   that should include it.
3. Mark its lines in the shared files: the Makefile, compose, `.env.example`, CI, `config/config.go`,
   `internal/app`, and the docs (README, CLAUDE.md, `docs/*.md`). Write prose so that it survives
   removal: list transports or features, don't count them.
4. `go test ./internal/scaffold/` checks that every file is claimed, that markers are balanced and
   name real features, that generating everything reproduces the template, and that a project
   without each feature mentions none of its `leaks`.
5. `make scaffold-check WITHOUT=<id>` proves the project without it passes `make pre-commit`.
