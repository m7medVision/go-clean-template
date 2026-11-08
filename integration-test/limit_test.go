package integration_test

import (
	"context"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/limiter"
	"github.com/m7medvision/go-clean-template/pkg/valkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const limitBudget = 2

func newLimiter(t *testing.T) repo.Limiter {
	t.Helper()

	valkeyURL := os.Getenv("VALKEY_URL")
	require.NotEmpty(t, valkeyURL, "VALKEY_URL is not set")

	ctx, cancel := context.WithTimeout(t.Context(), connectWait)
	defer cancel()

	client, err := valkey.New(ctx, valkeyURL)
	require.NoError(t, err)
	t.Cleanup(client.Close)

	l, err := limiter.New(client, entity.Limits{
		ClientAttempts:  limitBudget,
		AccountFailures: limitBudget,
		Window:          time.Minute,
	})
	require.NoError(t, err)

	return l
}

func TestLimiterRefusesTheTakeAfterTheClientBudget(t *testing.T) {
	l := newLimiter(t)

	ctx, cancel := context.WithTimeout(t.Context(), connectWait)
	defer cancel()

	scope := "_test:" + uuid.NewString()
	client := netip.MustParseAddr("192.0.2.1")

	for range limitBudget {
		got, err := l.TakeClient(ctx, scope, client)
		require.NoError(t, err)
		assert.True(t, got.Allowed)
	}

	got, err := l.TakeClient(ctx, scope, client)
	require.NoError(t, err)
	assert.False(t, got.Allowed)
	assert.Positive(t, got.RetryAfter)
}

func TestLimiterCountsAccountFailuresFromZeroAfterAClear(t *testing.T) {
	l := newLimiter(t)

	ctx, cancel := context.WithTimeout(t.Context(), connectWait)
	defer cancel()

	scope := "_test:" + uuid.NewString()
	account := uuid.NewString() + "@example.com"

	t.Cleanup(func() {
		assert.NoError(t, l.ClearAccount(context.Background(), scope, account))
	})

	for range limitBudget {
		got, err := l.CheckAccount(ctx, scope, account)
		require.NoError(t, err)
		assert.True(t, got.Allowed)

		require.NoError(t, l.TakeAccountFailure(ctx, scope, account))
	}

	got, err := l.CheckAccount(ctx, scope, account)
	require.NoError(t, err)
	assert.False(t, got.Allowed)
	assert.Positive(t, got.RetryAfter)

	require.NoError(t, l.ClearAccount(ctx, scope, account))
	require.NoError(t, l.TakeAccountFailure(ctx, scope, account))

	got, err = l.CheckAccount(ctx, scope, account)
	require.NoError(t, err)
	assert.True(t, got.Allowed)
}
