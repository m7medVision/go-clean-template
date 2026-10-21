-- name: CreateTask :exec
INSERT INTO tasks (id, user_id, title, description, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

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

-- name: UpdateTask :execrows
UPDATE tasks
SET title = $3, description = $4, status = $5, updated_at = $6
WHERE id = $1 AND user_id = $2;

-- name: DeleteTask :execrows
DELETE FROM tasks
WHERE id = $1 AND user_id = $2;

-- name: CountAllTasks :one
SELECT count(*) FROM tasks
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text);

-- name: ListAllTasks :many
SELECT * FROM tasks
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;
