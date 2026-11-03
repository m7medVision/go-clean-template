package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// InTxWith is InTx with the transaction opened by begin, so a test needs no database.
func InTxWith(ctx context.Context, begin func(context.Context) (pgx.Tx, error), fn func(context.Context) error) error {
	return inTx(ctx, begin, fn)
}
