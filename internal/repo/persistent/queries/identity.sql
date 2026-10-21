-- name: GetUserByIdentity :one
SELECT u.* FROM users u
JOIN user_identities i ON i.user_id = u.id
WHERE i.issuer = $1 AND i.subject = $2;

-- name: CreateUserWithIdentity :exec
-- One statement, so the user and its identity are stored together or not at all.
WITH new_user AS (
    INSERT INTO users (id, username, email, password_hash, created_at, updated_at)
    VALUES (sqlc.arg('id'), sqlc.arg('username'), sqlc.arg('email'), '', sqlc.arg('created_at'), sqlc.arg('created_at'))
    RETURNING id
)
INSERT INTO user_identities (issuer, subject, user_id, created_at)
SELECT sqlc.arg('issuer'), sqlc.arg('subject'), new_user.id, sqlc.arg('created_at') FROM new_user;
