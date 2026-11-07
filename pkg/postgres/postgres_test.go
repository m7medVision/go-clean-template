package postgres_test

import (
	"math"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port 1 is never a Postgres server, so every attempt must fail.
func TestNew_FailsWhenDatabaseIsUnreachable(t *testing.T) {
	t.Parallel()

	_, err := postgres.New(
		"postgres://user:pass@127.0.0.1:1/db?sslmode=disable",
		postgres.ConnAttempts(2),
		postgres.ConnTimeout(10*time.Millisecond),
	)

	require.Error(t, err)
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
