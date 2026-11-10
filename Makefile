ifneq ($(wildcard .env),)
include .env
export
else
$(warning WARNING: .env file not found! Using .env.example)
include .env.example
export
endif

BASE_STACK = docker compose -f docker-compose.yml
INTEGRATION_TEST_STACK = $(BASE_STACK) -f docker-compose-integration-test.yml
ALL_STACK = $(INTEGRATION_TEST_STACK)

# The compose services the app needs on the host: make compose-up starts COMPOSE_SERVICES, make dev
# (scripts/dev.sh reads DEV_SERVICES) and make dev-down add the rest.
COMPOSE_SERVICES = db valkey nats mongo
COMPOSE_SERVICES += mailpit
DEV_SERVICES = $(COMPOSE_SERVICES)
DEV_SERVICES += keycloak

# Top-level directories holding Go code.
GO_DIRS = $(patsubst %/,%,$(wildcard */))
# web/ is excluded: its node_modules can ship .go files. Every frontend target runs Vite+ inside web/.
GO_DIRS := $(filter-out web,$(GO_DIRS))
WEB = vp -C web
DOCS = vp -C site

# HELP =================================================================================================================
# This will output the help for each task
# thanks to https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
.PHONY: help

help: ## Display this help screen
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

compose-up: ### Run docker compose: the app's dependencies only (without backend and reverse proxy)
	$(BASE_STACK) up --build -d $(COMPOSE_SERVICES) && docker compose logs -f
.PHONY: compose-up

compose-up-all: ### Run docker compose (with backend and reverse proxy)
	$(BASE_STACK) up --build -d
.PHONY: compose-up-all

compose-up-integration-test: ### Run docker compose with integration test
	$(INTEGRATION_TEST_STACK) up --build --abort-on-container-exit --exit-code-from integration-test; exit_code=$$?; \
	$(INTEGRATION_TEST_STACK) down --remove-orphans; exit $$exit_code
.PHONY: compose-up-integration-test

compose-down: ### Down docker compose
	$(ALL_STACK) down --remove-orphans
.PHONY: compose-down

proto-v1: ### generate source files from proto
	protoc --go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		docs/proto/v1/*.proto
.PHONY: proto-v1

# The protobuf code is generated before anything that builds it.
run: proto-v1
pre-commit: proto-v1

graphql: ### generate GraphQL server code from schema (gqlgen)
	cd internal/controller/graphql && go tool gqlgen generate
.PHONY: graphql

openapi: ### export the REST OpenAPI document (Huma) to docs/openapi/v1.json, stating APP_VERSION
	mkdir -p docs/openapi
	go run ./cmd/openapi -version '$(APP_VERSION)' > docs/openapi/v1.json.tmp
	mv docs/openapi/v1.json.tmp docs/openapi/v1.json
.PHONY: openapi

web-deps: ### install frontend dependencies (Vite+, frozen lockfile)
	$(WEB) install --frozen-lockfile
.PHONY: web-deps

web-api: openapi ### generate the frontend REST types from docs/openapi/v1.json
	$(WEB) exec openapi-typescript ../docs/openapi/v1.json -o src/api/schema.gen.ts
.PHONY: web-api

web-graphql: ### generate the frontend GraphQL types from the gqlgen schema
	$(WEB) exec graphql-codegen --config codegen.ts
.PHONY: web-graphql

web-dev: ### run the Vite+ dev server on WEB_PORT, proxying the API to HTTP_PORT
	$(WEB) dev
.PHONY: web-dev

web-format: ### format and autofix frontend code (Oxfmt + Oxlint)
	$(WEB) check --fix
.PHONY: web-format

web-check: ### format check, lint, type check and unit-test the frontend
	$(WEB) check
	$(WEB) test
.PHONY: web-check

web-build: ### build the frontend into web/dist
	$(WEB) build
.PHONY: web-build

docs-deps: ### install the docs site dependencies (Vite+, frozen lockfile)
	$(DOCS) install --frozen-lockfile
.PHONY: docs-deps

docs-gen: ### generate the docs site's environment and make target tables from config/ and this Makefile
	$(DOCS) node scripts/gen.ts
.PHONY: docs-gen

docs-dev: docs-gen ### run the docs site on http://localhost:4321/go-clean-template/
	$(DOCS) exec astro dev
.PHONY: docs-dev

docs-build: docs-gen ### type-check and build the docs site into site/dist
	$(DOCS) exec astro check
	$(DOCS) exec astro build
.PHONY: docs-build

docs-format: ### format the docs site
	$(DOCS) fmt .
.PHONY: docs-format

docs-check: ### format check, unit-test and build the docs site
	$(DOCS) fmt --check .
	$(DOCS) test
	$(MAKE) docs-build
.PHONY: docs-check

web-e2e: ### run the Playwright suite against the running app and worker (make dev, or make run and make run-worker)
	@curl -fs http://localhost:$(HTTP_PORT)/healthz >/dev/null || { echo "web-e2e needs the app on HTTP_PORT=$(HTTP_PORT); start the stack first (make dev)"; exit 1; }
	$(WEB) exec playwright test -c e2e/playwright.config.ts
.PHONY: web-e2e

sqlc: ### generate type-safe query code from the migrations and the queries (sqlc)
	go tool sqlc generate
.PHONY: sqlc

deps: ### deps tidy + verify
	go mod tidy && go mod verify
.PHONY: deps

deps-audit: ### check dependencies vulnerabilities
	govulncheck ./...
.PHONY: deps-audit

fix-diff: ### Show code changes by `go fix`
	go fix -diff ./...
.PHONY: fix-diff

format: ### Run code formatter
	go fix ./...
	gofumpt -l -w $(GO_DIRS)
	gci write $(GO_DIRS) --skip-generated -s standard -s default
.PHONY: format

run: deps ### run the app for API v1
	go mod download && \
	CGO_ENABLED=0 go run -tags migrate ./cmd/app
.PHONY: run

projection-rebuild: ### rebuild one projection's read model from the event store: make projection-rebuild name=task
	@test -n "$(name)" || { echo 'usage: make projection-rebuild name=<projector>'; exit 2; }
	go run ./cmd/worker -rebuild '$(name)'
.PHONY: projection-rebuild

# The generator is its own module (tools/gen): nothing in the app imports it, and no binary or image
# includes it. Optional arguments are passed on only when set.
GEN = go -C tools/gen run .
GEN_OPTS = $(if $(transports),--transports '$(transports)') $(if $(dry-run),--dry-run)
# After the generator writes, the regenerators its output needs, then a build; a dry run runs none.
# This is the one list of them: the generator names none. The first failure stops them. The undo
# the generator printed restores the tree, whatever they changed included: rule R3 had no tracked
# file changed before the write, and the undo leaves the untracked files that were there alone.
# Their output scrolls the generator's checklist and undo out of view, so the last line points back
# to them.
GEN_REGENERATE = $(if $(dry-run),,$(MAKE) proto-v1 graphql sqlc mock openapi web-api web-graphql format && go build ./... \
	|| { echo 'gen: a regenerator or the build failed: run the undo printed above to restore the tree' >&2; exit 1; }; \
	echo 'gen: done. The next steps and the undo are printed above the output of the regenerators.')

new-domain: ### generate a domain with CRUD stubs: make new-domain name=invoice [plural=] [transports=rest,grpc,graphql] [dry-run=1]
	$(GEN) new-domain --root '$(CURDIR)' --name '$(name)' $(if $(plural),--plural '$(plural)') $(GEN_OPTS)
	$(GEN_REGENERATE)
.PHONY: new-domain

new-usecase: ### add a command or query stub: make new-usecase domain=task name=archive-task kind=command [plural=] [method=] [path=] [transports=] [dry-run=1]
	$(GEN) new-usecase --root '$(CURDIR)' --domain '$(domain)' --name '$(name)' --kind '$(kind)' $(if $(plural),--plural '$(plural)') \
		$(if $(method),--method '$(method)') $(if $(path),--path '$(path)') $(GEN_OPTS)
	$(GEN_REGENERATE)
.PHONY: new-usecase

gen-check: ### check the generator: lint and format (the root .golangci.yml), unit and golden tests, tidy module
	cd tools/gen && golangci-lint run --config ../../.golangci.yml ./...
	go -C tools/gen test -race ./...
	go -C tools/gen mod tidy -diff
.PHONY: gen-check

gen-e2e: ### generate a domain into a throwaway worktree of the working tree, then lint and test it there
	./scripts/gen-e2e.sh
.PHONY: gen-e2e

run-worker: ### run the queue worker (start the app first: it applies the schema)
	CGO_ENABLED=0 go run ./cmd/worker
.PHONY: run-worker

dev: ### run everything in dev mode in tmux: DEV_SERVICES in docker, the rest with hot reload
	./scripts/dev.sh
.PHONY: dev

dev-down: ### stop make dev: close its tmux session, stop DEV_SERVICES
	-tmux kill-session -t =$(notdir $(CURDIR)) 2>/dev/null
	$(BASE_STACK) stop $(DEV_SERVICES)
.PHONY: dev-down

docker-rm-volume: ### remove the Postgres volume (stops the stack)
	$(BASE_STACK) down --volumes --remove-orphans
.PHONY: docker-rm-volume

linter-golangci: ### check by golangci linter
	golangci-lint run
.PHONY: linter-golangci

linter-hadolint: ### check by hadolint linter
	git ls-files --exclude='Dockerfile*' --ignored | xargs hadolint
.PHONY: linter-hadolint

linter-dotenv: ### check by dotenv linter
	dotenv-linter
.PHONY: linter-dotenv

test: ### run test
	go test -v -race -covermode atomic -coverprofile=coverage.txt ./config/... ./internal/... ./pkg/...
.PHONY: test

integration-test: ### run integration-test
	go clean -testcache && go test -v ./integration-test/...
.PHONY: integration-test

mock: ### run mockgen: one mock file per contract file, from its go:generate directive
	rm -rf ./internal/repo/repomock
	go generate ./internal/repo
.PHONY: mock

migrate-up: ### apply every pending migration to PG_URL's database
	go run ./cmd/migrate -url '$(PG_URL)' up
.PHONY: migrate-up

migrate-down: ### roll PG_URL's database back to version=<version> (0 empties it)
	@test -n "$(version)" || { echo "usage: make migrate-down version=<version>"; exit 1; }
	go run ./cmd/migrate -url '$(PG_URL)' down-to $(version)
.PHONY: migrate-down

migrate-status: ### list the migrations and whether PG_URL's database has them
	go run ./cmd/migrate -url '$(PG_URL)' status
.PHONY: migrate-status

migrate-new: ### write an empty migration, internal/app/migrations/<timestamp>_<name>.sql
	@test -n "$(name)" || { echo "usage: make migrate-new name=<name>"; exit 1; }
	go run ./cmd/migrate create $(name)
.PHONY: migrate-new

schema-roundtrip: ### every migration up, down to zero and up again in a throwaway Postgres; no merged migration edited
	./scripts/schema-roundtrip.sh $(BASE_REF)
.PHONY: schema-roundtrip

bin-deps: ### install tools
	go install tool
.PHONY: bin-deps

# svu names the version from the commits since the last tag; --v0 keeps a breaking change below 1.0
# a minor bump. A tag HEAD does not contain means a later merge was released first, and covers HEAD.
release-version: ### print the next release version from the commits since the last tag, or nothing
	@next=$$(go tool svu next --v0) && current=$$(go tool svu current) || exit 1; \
	if [ -n "$$(git tag --list 'v*' --no-merged HEAD)" ]; then \
		echo "release: nothing to release: a newer release already covers this commit" >&2; \
	elif [ "$$next" = "$$current" ]; then \
		echo "release: nothing to release: no feat, fix or breaking commit since $$current" >&2; \
	else \
		echo "$$next"; \
	fi
.PHONY: release-version

pre-commit: deps mock format linter-golangci test gen-check ### run pre-commit
pre-commit: web-api web-graphql web-format web-check
.PHONY: pre-commit
