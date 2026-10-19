-- name: SendQueueMessage :one
INSERT INTO queue_messages (queue, body)
VALUES ($1, $2)
RETURNING id;

-- ReadQueueMessages claims up to qty visible messages and hides them for vt_seconds in one
-- statement. SKIP LOCKED lets concurrent workers claim disjoint batches without waiting.
-- name: ReadQueueMessages :many
WITH due AS (
  SELECT q.id FROM queue_messages AS q
  WHERE q.queue = sqlc.arg('queue') AND q.visible_at <= now()
  ORDER BY q.id
  LIMIT sqlc.arg('qty')::integer
  FOR UPDATE SKIP LOCKED
)
UPDATE queue_messages AS m
SET visible_at = now() + make_interval(secs => sqlc.arg('vt_seconds')::double precision),
    read_count = m.read_count + 1
FROM due
WHERE m.id = due.id
RETURNING m.id, m.queue, m.body, m.read_count, m.enqueued_at, m.visible_at;

-- name: DeleteQueueMessage :exec
DELETE FROM queue_messages
WHERE id = $1;
