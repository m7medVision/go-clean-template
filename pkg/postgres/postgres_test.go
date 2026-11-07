package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errRefused = errors.New("dial tcp: connection refused")

// Port 1 is never a Postgres server, so every attempt fails, and New keeps trying until ctx ends.
// It returns then, without waiting out the pause before the next attempt.
func TestNew_GivesUpWhenTheContextEnds(t *testing.T) {
	t.Parallel()

	const deadline = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(t.Context(), deadline)
	defer cancel()

	start := time.Now()

	_, err := postgres.New(ctx, "postgres://user:pass@127.0.0.1:1/db?sslmode=disable", 1)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 10*deadline, "New waited past its context")
}

// A database that refuses the login, or has no such database, answers the same on every attempt.
func TestNew_RetriesOnlyWhatMaySucceedLater(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "an unreachable database", err: errRefused, want: true},
		{name: "a database still starting", err: &pgconn.PgError{Code: "57P03"}, want: true},
		{name: "too many connections", err: &pgconn.PgError{Code: "53300"}, want: true},
		{name: "a wrong password", err: fmt.Errorf("connect: %w", &pgconn.PgError{Code: "28P01"}), want: false},
		{name: "a login pg_hba.conf refuses", err: &pgconn.PgError{Code: "28000"}, want: false},
		{name: "no such database", err: &pgconn.PgError{Code: "3D000"}, want: false},
	} {
		assert.Equal(t, tc.want, postgres.Retryable(tc.err), tc.name)
	}
}

// A size past the int32 range is the largest pool, not one whose high bits were cut off.
func TestThePoolSizeIsClampedToWhatPgxpoolTakes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		size int
		want int32
	}{
		{size: 10, want: 10},
		{size: 0, want: 1},
		{size: -1, want: 1},
		{size: math.MaxInt32, want: math.MaxInt32},
		{size: math.MaxInt32 + 1, want: math.MaxInt32},
	} {
		assert.Equal(t, tc.want, postgres.PoolSize(tc.size), "size %d", tc.size)
	}
}
