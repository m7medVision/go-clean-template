package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// InTxWith is InTx with the transaction opened by begin, so a test needs no database.
func InTxWith(ctx context.Context, begin func(context.Context) (pgx.Tx, error), fn func(context.Context) error) error {
	return inTx(ctx, begin, fn)
}

// PoolSize is the MaxConns a pool of size connections opens with.
func PoolSize(size int) int32 {
	return safeIntToInt32(size)
}

// Retryable reports whether New tries again after err.
func Retryable(err error) bool {
	return retryable(err)
}
