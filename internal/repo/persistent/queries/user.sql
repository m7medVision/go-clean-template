-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- The user projector's writes. Each leaves an existing row alone, so replaying a user's first event
-- (an import, a rebuild) never overwrites it.

-- name: InsertUserRow :exec
INSERT INTO users (id, username, email, password_hash, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO NOTHING;

-- name: InsertIdentityRow :exec
INSERT INTO user_identities (issuer, subject, user_id, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (issuer, subject) DO NOTHING;

-- name: ListEveryUser :many
SELECT * FROM users
ORDER BY created_at, id;

-- name: ListEveryIdentity :many
SELECT * FROM user_identities
ORDER BY user_id, created_at;
