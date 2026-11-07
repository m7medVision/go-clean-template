package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is what queries run on: the pool, or the transaction a context carries. sqlc's generated
// queries take the same interface.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

// InTx runs fn in one transaction, carried by the context fn receives: every repository that
// builds its queries on Executor(ctx) joins it. The transaction commits when fn returns nil and
// rolls back when fn fails or panics. Inside a transaction already, InTx joins it instead of
// opening a second one, and the outermost call decides the outcome.
func (p *Postgres) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return inTx(ctx, p.pool.Begin, fn)
}

// Executor returns the transaction ctx carries, or the pool when it carries none.
func (p *Postgres) Executor(ctx context.Context) DBTX {
	if tx, ok := txFrom(ctx); ok {
		return tx
	}

	return p.pool
}

// txFrom returns the transaction ctx carries.
func txFrom(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)

	return tx, ok
}

func inTx(
	ctx context.Context, begin func(context.Context) (pgx.Tx, error), fn func(context.Context) error,
) error {
	if _, ok := txFrom(ctx); ok {
		return fn(ctx)
	}

	tx, err := begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres - InTx - begin: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // the panic goes on; a failed rollback's connection is discarded by the pool

			panic(p)
		}
	}()

	if err = fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		if rbErr := tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("postgres - InTx - rollback: %w", rbErr))
		}

		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres - InTx - commit: %w", err)
	}

	return nil
}
