# Plan: goose

Spec: [goose.md](goose.md) (approved 2026-11-09). One PR, `feat/goose`, off master `8ec12ac`.

1. **Baseline.** `internal/app/migrations/00001_baseline.sql` is the rendered `schema.sql` as it
   stood (the `public` schema lines dropped: goose runs inside it), with a `Down` that drops every
   table. `make sqlc` reading the migrations must produce no diff in `queries/*.go`: that proves the
   baseline is the old schema.
2. **Runner.** `pkg/migrate` (goose `Provider` on an `embed.FS` or a directory, the Postgres session
   locker, pgx's `database/sql` driver); `internal/app/migrate.go` calls it at startup with the
   `migrate` tag; `cmd/migrate` gives the `make migrate-*` targets and the round trip a CLI.
3. **Tooling.** `sqlc.yaml` reads `internal/app/migrations`; the Makefile loses the Atlas render,
   `schema-apply` and the Atlas install; the Dockerfile loses the Atlas binary and `/tmp`;
   `scripts/schema-roundtrip.sh` checks no merged migration changed, then up / down to 0 / up.
4. **CI.** No Atlas install anywhere; the sqlc job needs no Postgres; the round trip needs Go; the
   path filters follow the migrations.
5. **Generator.** The domain table template becomes `{{.Version}}_create_<plural>.sql`; `Data.Version`
   is the UTC timestamp (fixed in the goldens); rule R1 refuses a name whose create migration exists.
6. **Docs.** Every page that said HCL or Atlas.

Risk: a migration that runs on master but not on an old database built by Atlas. Accepted: the
spec says reset the database. Verification: `make format`, `make linter-golangci`, `make test`,
`make gen-check`, `make gen-e2e`, `make schema-roundtrip`, then CI (integration included).
