-- LockEventStore serializes appends for the rest of the transaction, so positions commit in the
-- order they are assigned and a reader that has passed a position never sees an earlier one
-- commit later.
-- name: LockEventStore :exec
SELECT pg_advisory_xact_lock(sqlc.arg('key')::bigint);

-- name: StreamVersion :one
SELECT COALESCE(MAX(stream_version), 0)::integer
FROM events
WHERE stream_id = $1;

-- name: AppendEvent :one
INSERT INTO events (stream_id, stream_version, type, schema_version, payload, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING position;

-- name: ReadStream :many
SELECT position, stream_id, stream_version, type, schema_version, payload, metadata, recorded_at
FROM events
WHERE stream_id = $1 AND stream_version > sqlc.arg('after_version')
ORDER BY stream_version;

-- ReadAll returns the events after a position, for the projection runner. Streams whose ID starts
-- with an underscore are scratch streams (the store's own tests write them): only a reader that
-- asks for them gets them, and the worker's never does.
-- name: ReadAll :many
SELECT position, stream_id, stream_version, type, schema_version, payload, metadata, recorded_at
FROM events
WHERE position > sqlc.arg('after_position')
  AND (sqlc.arg('include_scratch')::boolean OR stream_id NOT LIKE '\_%')
ORDER BY position
LIMIT sqlc.arg('max_events');

-- name: LastPosition :one
SELECT COALESCE(MAX(position), 0)::bigint
FROM events
WHERE stream_id NOT LIKE '\_%';

-- NotifyEvents wakes the projection runners; Postgres delivers it when the transaction commits.
-- name: NotifyEvents :exec
SELECT pg_notify('events', '');

-- name: GetSnapshot :one
SELECT stream_version, schema_version, payload
FROM snapshots
WHERE stream_id = $1;

-- name: PutSnapshot :exec
INSERT INTO snapshots (stream_id, stream_version, schema_version, payload)
VALUES ($1, $2, $3, $4)
ON CONFLICT (stream_id) DO UPDATE
SET stream_version = EXCLUDED.stream_version,
    schema_version = EXCLUDED.schema_version,
    payload = EXCLUDED.payload,
    taken_at = now();

-- InsertClaim reserves a value for owner_id. It affects no row when another owner holds the
-- value, and one row when the value is new or owner_id holds it already.
-- name: InsertClaim :execrows
INSERT INTO claims (scope, value, owner_id)
VALUES ($1, $2, $3)
ON CONFLICT (scope, value) DO UPDATE
SET owner_id = claims.owner_id
WHERE claims.owner_id = EXCLUDED.owner_id;

-- name: ReleaseClaim :exec
DELETE FROM claims
WHERE scope = $1 AND value = $2 AND owner_id = $3;

-- name: ClaimOwner :one
SELECT owner_id
FROM claims
WHERE scope = $1 AND value = $2;

-- name: EnsureCheckpoint :exec
INSERT INTO projection_checkpoints (name)
VALUES ($1)
ON CONFLICT (name) DO NOTHING;

-- name: GetCheckpoint :one
SELECT position
FROM projection_checkpoints
WHERE name = $1;

-- AdvanceCheckpoint moves a checkpoint only from the position the caller read, so a reset by a
-- rebuild in between makes the advance affect no row instead of overwriting it.
-- name: AdvanceCheckpoint :execrows
UPDATE projection_checkpoints
SET position = sqlc.arg('to_position'), updated_at = now()
WHERE name = sqlc.arg('name') AND position = sqlc.arg('from_position');

-- name: ResetCheckpoint :exec
UPDATE projection_checkpoints
SET position = 0, updated_at = now()
WHERE name = $1;

-- CheckpointsReached counts the named checkpoints at or past position.
-- name: CheckpointsReached :one
SELECT count(*)::integer
FROM projection_checkpoints
WHERE name = ANY(sqlc.arg('names')::text[]) AND position >= sqlc.arg('min_position');

-- CheckpointLag lists how many events each checkpoint is behind the last one stored.
-- name: CheckpointLag :many
SELECT c.name, (
  (SELECT COALESCE(MAX(e.position), 0) FROM events AS e WHERE e.stream_id NOT LIKE '\_%') - c.position
)::bigint AS lag
FROM projection_checkpoints AS c
ORDER BY c.name;

-- TryLockSubscriber takes a subscriber's lock until the transaction ends, so one runner at a time
-- handles it; false when another runner holds it.
-- name: TryLockSubscriber :one
SELECT pg_try_advisory_xact_lock(hashtext(sqlc.arg('name')::text)::bigint);
