-- name: ListPasskeysByUser :many
SELECT * FROM webauthn_credentials
WHERE user_id = $1
ORDER BY created_at, id;

-- name: GetPasskey :one
SELECT * FROM webauthn_credentials
WHERE id = $1;

-- name: BurnChallenge :one
INSERT INTO webauthn_used_challenges (challenge_hash, expires_at)
VALUES ($1, $2)
ON CONFLICT (challenge_hash) DO NOTHING
RETURNING challenge_hash;

-- name: DeleteExpiredChallenges :exec
DELETE FROM webauthn_used_challenges
WHERE expires_at < now();

-- The passkey projector's writes. InsertPasskeyRow leaves an existing row alone, so replaying a
-- passkey's first event (an import, a rebuild) never overwrites it.

-- name: InsertPasskeyRow :exec
INSERT INTO webauthn_credentials (id, user_id, name, credential, created_at, last_used_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO NOTHING;

-- name: SetPasskeyName :exec
UPDATE webauthn_credentials
SET name = $2
WHERE id = $1;

-- name: SetPasskeyUsage :exec
UPDATE webauthn_credentials
SET credential = $2, last_used_at = $3
WHERE id = $1;

-- name: DeletePasskeyRow :exec
DELETE FROM webauthn_credentials
WHERE id = $1;

-- name: ListEveryPasskey :many
SELECT * FROM webauthn_credentials
ORDER BY created_at, id;
