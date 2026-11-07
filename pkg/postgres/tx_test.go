package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errWork   = errors.New("work failed")
	errBegin  = errors.New("begin failed")
	errCommit = errors.New("commit failed")
)

// fakeTx records how its transaction ended.
type fakeTx struct {
	pgx.Tx

	commitErr  error
	committed  bool
	rolledBack bool
}

func (tx *fakeTx) Commit(context.Context) error {
	tx.committed = true

	return tx.commitErr
}

func (tx *fakeTx) Rollback(context.Context) error {
	tx.rolledBack = true

	return nil
}

// beginner hands out tx and counts how often it was asked.
type beginner struct {
	tx    *fakeTx
	err   error
	calls int
}

func (b *beginner) begin(context.Context) (pgx.Tx, error) {
	b.calls++
	if b.err != nil {
		return nil, b.err
	}

	return b.tx, nil
}

func TestInTxCommitsWhenTheWorkSucceeds(t *testing.T) {
	t.Parallel()

	b := &beginner{tx: &fakeTx{}}

	err := postgres.InTxWith(context.Background(), b.begin, func(context.Context) error { return nil })

	require.NoError(t, err)
	assert.True(t, b.tx.committed)
	assert.False(t, b.tx.rolledBack)
}

func TestInTxRollsBackWhenTheWorkFails(t *testing.T) {
	t.Parallel()

	b := &beginner{tx: &fakeTx{}}

	err := postgres.InTxWith(context.Background(), b.begin, func(context.Context) error { return errWork })

	require.ErrorIs(t, err, errWork)
	assert.False(t, b.tx.committed)
	assert.True(t, b.tx.rolledBack)
}

func TestInTxRollsBackAndPanicsAgainWhenTheWorkPanics(t *testing.T) {
	t.Parallel()

	b := &beginner{tx: &fakeTx{}}

	assert.PanicsWithValue(t, "boom", func() {
		_ = postgres.InTxWith(context.Background(), b.begin, func(context.Context) error { panic("boom") }) //nolint:errcheck // the call panics before it returns
	})
	assert.True(t, b.tx.rolledBack)
}

func TestInTxReturnsABeginFailure(t *testing.T) {
	t.Parallel()

	b := &beginner{err: errBegin}

	err := postgres.InTxWith(context.Background(), b.begin, func(context.Context) error { return nil })

	require.ErrorIs(t, err, errBegin)
}

func TestInTxReturnsACommitFailure(t *testing.T) {
	t.Parallel()

	b := &beginner{tx: &fakeTx{commitErr: errCommit}}

	err := postgres.InTxWith(context.Background(), b.begin, func(context.Context) error { return nil })

	require.ErrorIs(t, err, errCommit)
}

func TestANestedInTxJoinsTheOuterTransaction(t *testing.T) {
	t.Parallel()

	b := &beginner{tx: &fakeTx{}}

	var inner pgx.Tx

	err := postgres.InTxWith(context.Background(), b.begin, func(ctx context.Context) error {
		return postgres.InTxWith(ctx, b.begin, func(ctx context.Context) error {
			tx, ok := postgres.TxFrom(ctx)
			require.True(t, ok)

			inner = tx

			return nil
		})
	})

	require.NoError(t, err)
	assert.Equal(t, 1, b.calls)
	assert.Same(t, b.tx, inner)
}

func TestExecutorIsTheTransactionTheContextCarries(t *testing.T) {
	t.Parallel()

	b := &beginner{tx: &fakeTx{}}
	pg := &postgres.Postgres{}

	err := postgres.InTxWith(context.Background(), b.begin, func(ctx context.Context) error {
		assert.Same(t, b.tx, pg.Executor(ctx))

		return nil
	})

	require.NoError(t, err)
}

func TestExecutorIsThePoolOutsideATransaction(t *testing.T) {
	t.Parallel()

	pg := &postgres.Postgres{}

	_, ok := postgres.TxFrom(context.Background())

	assert.False(t, ok)
	assert.IsType(t, (*pgxpool.Pool)(nil), pg.Executor(context.Background()))
}
