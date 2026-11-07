-- InsertConversionRow is the exchange projector's write. It leaves an existing conversion alone,
-- so replaying one (an import, a rebuild) never duplicates it.
-- name: InsertConversionRow :exec
INSERT INTO exchange_history
  (conversion_id, user_id, from_currency, to_currency, amount, rate, result, rate_date, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (conversion_id) DO NOTHING;

-- name: GetConversion :one
SELECT from_currency, to_currency, amount, rate, result, rate_date FROM exchange_history
WHERE conversion_id = $1 AND user_id = $2;

-- ListEveryExchange is what the backfill imports, oldest first, so a rebuild keeps the order.
-- name: ListEveryExchange :many
SELECT * FROM exchange_history
ORDER BY id;

-- name: SetConversionID :exec
UPDATE exchange_history
SET conversion_id = $2
WHERE id = $1 AND conversion_id IS NULL;

-- name: CountExchanges :one
SELECT count(*) FROM exchange_history
WHERE user_id = $1;

-- ListExchanges pages by when each conversion happened, newest first.
-- name: ListExchanges :many
SELECT from_currency, to_currency, amount, rate, result, rate_date FROM exchange_history
WHERE user_id = sqlc.arg('user_id')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;
