-- name: ListHistory :many
SELECT source, destination, original, translation FROM history
WHERE user_id = $1;

-- name: CreateHistory :exec
INSERT INTO history (user_id, source, destination, original, translation)
VALUES ($1, $2, $3, $4, $5);
