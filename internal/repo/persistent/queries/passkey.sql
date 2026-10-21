-- name: StorePasskey :exec
INSERT INTO webauthn_credentials (id, user_id, name, credential, created_at, last_used_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListPasskeysByUser :many
SELECT * FROM webauthn_credentials
WHERE user_id = $1
ORDER BY created_at, id;

-- name: GetPasskey :one
SELECT * FROM webauthn_credentials
WHERE id = $1;

-- name: UpdatePasskeyUsage :execrows
UPDATE webauthn_credentials
SET credential = $2, last_used_at = $3
WHERE id = $1;

-- name: RenamePasskey :one
UPDATE webauthn_credentials
SET name = $3
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeletePasskey :execrows
DELETE FROM webauthn_credentials
WHERE id = $1 AND user_id = $2;

-- name: BurnChallenge :one
INSERT INTO webauthn_used_challenges (challenge_hash, expires_at)
VALUES ($1, $2)
ON CONFLICT (challenge_hash) DO NOTHING
RETURNING challenge_hash;

-- name: DeleteExpiredChallenges :exec
DELETE FROM webauthn_used_challenges
WHERE expires_at < now();
