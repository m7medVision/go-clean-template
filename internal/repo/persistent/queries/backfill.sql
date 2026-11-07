-- HasMarker reports whether eventstore.Backfill has imported domain's rows.
-- name: HasMarker :one
SELECT EXISTS (SELECT 1 FROM backfill_markers WHERE domain = $1);

-- name: PutMarker :exec
INSERT INTO backfill_markers (domain)
VALUES ($1);

-- ImportedDomains returns the domains among domains that have been imported.
-- name: ImportedDomains :many
SELECT domain
FROM backfill_markers
WHERE domain = ANY(sqlc.arg('domains')::text[]);

-- MoveLegacyMarkers moves the markers v0.3 kept among the checkpoints, named _backfill/<domain>,
-- to backfill_markers, so a database upgraded from v0.3 does not import again. Once they have
-- moved, it finds none.
-- name: MoveLegacyMarkers :exec
WITH legacy AS (
  DELETE FROM projection_checkpoints
  WHERE name LIKE '\_backfill/%'
  RETURNING substr(name, length('_backfill/') + 1) AS domain, updated_at
)
INSERT INTO backfill_markers (domain, imported_at)
SELECT legacy.domain, legacy.updated_at
FROM legacy
ON CONFLICT (domain) DO NOTHING;
