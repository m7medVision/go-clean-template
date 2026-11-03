-- name: GetTask :one
SELECT * FROM tasks
WHERE id = $1;

-- name: CountTasks :one
SELECT count(*) FROM tasks
WHERE user_id = sqlc.arg('user_id')
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text);

-- name: ListTasks :many
SELECT * FROM tasks
WHERE user_id = sqlc.arg('user_id')
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

-- name: CountAllTasks :one
SELECT count(*) FROM tasks
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text);

-- name: ListAllTasks :many
SELECT * FROM tasks
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

-- The task projector's writes. InsertTaskRow leaves an existing row alone, so replaying a task's
-- first event (an import, a rebuild) never overwrites it.

-- name: InsertTaskRow :exec
INSERT INTO tasks (id, user_id, title, description, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO NOTHING;

-- name: SetTaskText :exec
UPDATE tasks
SET title = $2, description = $3, updated_at = $4
WHERE id = $1;

-- name: SetTaskStatus :exec
UPDATE tasks
SET status = $2, updated_at = $3
WHERE id = $1;

-- name: DeleteTaskRow :exec
DELETE FROM tasks
WHERE id = $1;

-- name: ListEveryTask :many
SELECT * FROM tasks
ORDER BY created_at, id;
