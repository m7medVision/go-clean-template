# Spec: versioned migrations with goose

Module id: `goose` · first of the observability and messaging work (see
[README.md](README.md#migrations)) · Depends on nothing · Blocks `nats-queue` · Status:
**done: merged 2026-11-09 with #56 (stacked under it), released in v0.6.0**

Paths below are checked against `feat/valkey` at `79e68ed` on 2026-11-08.

## Objective

The schema moves from Atlas's declarative HCL to **versioned SQL migrations run by goose**. Atlas
diffs the HCL against the live database and drops what the HCL does not declare. That rules out
tables another tool owns (Watermill's outbox), data migrations, and anything Atlas's free tier
does not express. With goose a migration is a SQL file that says exactly what runs, in order, once.
When it is done:

- `internal/app/migrations/<timestamp>_<name>.sql` holds the schema, with `-- +goose Up` and
  `-- +goose Down` sections; the first file is the baseline: today's schema exactly;
- the app applies pending migrations at startup with the `migrate` build tag, as it applies the HCL
  today, under goose's Postgres advisory lock, so two app instances never run them together;
- **sqlc reads the migrations directory directly**, so `schema.sql`, its render step, `ATLAS_DEV_URL`
  and Docker for `make sqlc` all go;
- `make migrate-new name=<name>`, `make migrate-up`, `make migrate-down`, `make migrate-status`
  replace `make schema-apply`;
- the round-trip check becomes: every migration in the branch goes up, down and up again on a
  throwaway Postgres;
- `make new-domain` writes a migration file instead of an HCL file;
- Atlas, `atlasexec`, the HCL files and the Atlas install step are gone.

**Who it is for:** a developer starting from the template, who needs to add a column, backfill it
and drop the old one in steps, and to share the database with a library that owns its own tables.

## Tech stack

| Piece | Version | Why |
| --- | --- | --- |
| `github.com/pressly/goose/v3`, library only, pinned in the plan | latest v3 | embedded migrations (`embed.FS`), the `Provider` API, a Postgres session locker; no CLI needed in the image |
| sqlc | the one pinned in `go.mod` | reads goose files as its schema, ignoring `Down` sections |

## Design (decided)

1. **SQL migrations, no Go migrations** to start. A data change is SQL in a migration.
2. **Applied at app startup** with `-tags migrate` (`make run`, `make dev`, the Dockerfile), never by
   the worker, as today.
3. **Never edit a merged migration.** A change is a new file. CI fails if a migration on master
   changed (checksum of the files that exist on the base ref).
4. **Every migration has a `Down`** that undoes its `Up`, proven by the round trip.
5. **Still one database**: no dev database, nothing diffs against a second Postgres.
6. **Timestamped file names** (decided 2026-11-08): `20261108120000_add_archived_at.sql`, so two
   branches never take the same number. The baseline is `00001_baseline.sql`; `make migrate-new`
   writes the timestamp.
7. **No upgrade path for existing databases** (decided 2026-11-08): this is a template people build
   on, so a database made by Atlas is reset (`make compose-down`, then up). The release notes say so.

```sql
-- +goose Up
ALTER TABLE tasks ADD COLUMN archived_at timestamptz;

-- +goose Down
ALTER TABLE tasks DROP COLUMN archived_at;
```

```go
func migrate(ctx context.Context, db *sql.DB) error {
	sub, _ := fs.Sub(migrationsFS, "migrations")
	locker, err := lock.NewPostgresSessionLocker()
	...
	p, err := goose.NewProvider(goose.DialectPostgres, db, sub, goose.WithSessionLocker(locker))
	...
	_, err = p.Up(ctx)
	return err
}
```

## Project structure

| Where | Change |
| --- | --- |
| `internal/app/schema/*.hcl` | removed |
| `internal/app/migrations/00001_baseline.sql` | new: today's rendered `schema.sql`, plus its `Down` |
| `internal/app/migrate.go` | goose `Provider` on an embedded FS instead of `atlasexec` |
| `internal/repo/persistent/queries/schema/` | removed; `sqlc.yaml` `schema:` points at `internal/app/migrations` |
| `Makefile` | `migrate-*` targets; `sqlc` loses its render step; `schema-apply`, `bin-deps`' Atlas line go |
| `scripts/schema-roundtrip.sh` | up/down/up over goose; no Atlas |
| `.github/workflows/ci.yml` | no Atlas install; the path filters on `internal/app/migrations/**`; the "merged migration changed" check |
| `tools/gen` (domain plan, templates, rules, goldens) | a migration file `<timestamp>_create_<plural>.sql` instead of `<plural>.hcl` |
| `docs/database.md` (or the Database section where it lives), `docs/development.md`, `docs/generator.md`, `docs/architecture.md`, `CLAUDE.md` | the Database section rewritten; "edit the HCL" becomes "add a migration" |

## Testing strategy

- **Round trip** (`make schema-roundtrip`, and CI): on an empty Postgres, up to the base ref's last
  migration, then the branch's, down to the base, up again; `goose status` clean.
- **Baseline equals today:** `pg_dump --schema-only` after the baseline equals one after today's
  Atlas apply (once, in the PR, by a script, not kept).
- **Unit:** `make sqlc` output unchanged by the switch (no diff in `queries/*.go`).
- **Generator:** `make gen-check` and `make gen-e2e` with the migration template.
- **Integration:** the whole suite on a database built by goose.

## Boundaries

- **Always:** a `Down` per migration; the round trip before a migration PR; one file per change.
- **Ask first:** Go migrations; running migrations from the worker or a separate job; squashing.
- **Never:** editing a migration on master; Atlas or another diffing tool back; a second database.

## Success criteria

1. A fresh database built by goose has exactly today's schema.
2. `make sqlc` runs without Docker and produces no diff.
3. Two app instances starting together apply each migration once.
4. `make new-domain` writes a numbered migration, and `make gen-e2e` passes.
5. No `atlas` in the repo, the image, `go.mod` or CI; checks and CI green.

## Open questions

None. `make sqlc` keeps its name (decided 2026-11-08): it still says what it runs.
