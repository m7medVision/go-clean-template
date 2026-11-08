# Tasks: goose

- [x] Baseline migration from the rendered schema; `make sqlc` shows no diff.
- [x] `pkg/migrate`, `internal/app/migrate.go`, `cmd/migrate`, its test.
- [x] Makefile, `sqlc.yaml`, Dockerfile, `.air.app.toml`, `.env.example`, round-trip script.
- [x] CI: Atlas removed, filters and the round-trip job updated.
- [x] Generator: migration template, `Data.Version`, rule R1, goldens.
- [x] Docs: CLAUDE.md, README, architecture, configuration, development, generator, passkeys,
      request-flow, sso, worker.
- [ ] `make gen-e2e`; CI green on the PR.
