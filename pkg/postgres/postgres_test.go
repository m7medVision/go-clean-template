package postgres_test

import (
	"testing"
	"time"

	"github.com/evrone/go-clean-template/pkg/postgres"
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
