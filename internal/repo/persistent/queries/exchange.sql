-- name: CreateExchange :exec
INSERT INTO exchange_history (user_id, from_currency, to_currency, amount, rate, result, rate_date)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: CountExchanges :one
SELECT count(*) FROM exchange_history
WHERE user_id = $1;

-- name: ListExchanges :many
SELECT from_currency, to_currency, amount, rate, result, rate_date FROM exchange_history
WHERE user_id = sqlc.arg('user_id')
ORDER BY id DESC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;
