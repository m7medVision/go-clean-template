package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"strconv"
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

const (
	limitBudget        = 2
	appAccountFailures = 5
	wrongPassword      = testPassword + "-wrong"
)

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

func registerForLimit(t *testing.T) string {
	t.Helper()

	name := sanitizeTestName(t)
	email := name + "@test.com"

	resp := registerUser(t, name, email, testPassword)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	return email
}

func signIn(t *testing.T, email, password string) (int, http.Header) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)

	resp, err := doWebRequestWithTimeout(ctx, http.MethodPost, basePathV1+"/auth/login", bytes.NewBufferString(body))
	require.NoError(t, err)

	defer resp.Body.Close()

	return resp.StatusCode, resp.Header
}

func TestHTTPLoginRefusesTheAccountAfterItsFailureBudget(t *testing.T) {
	email := registerForLimit(t)

	for range appAccountFailures {
		status, _ := signIn(t, email, wrongPassword)
		require.Equal(t, http.StatusUnauthorized, status)
	}

	status, header := signIn(t, email, testPassword)
	require.Equal(t, http.StatusTooManyRequests, status)

	retryAfter, err := strconv.Atoi(header.Get("Retry-After"))
	require.NoError(t, err)
	assert.Positive(t, retryAfter)
}

func TestHTTPLoginSuccessClearsTheAccountFailures(t *testing.T) {
	email := registerForLimit(t)

	for range appAccountFailures - 1 {
		status, _ := signIn(t, email, wrongPassword)
		require.Equal(t, http.StatusUnauthorized, status)
	}

	status, _ := signIn(t, email, testPassword)
	require.Equal(t, http.StatusOK, status)

	for range appAccountFailures {
		status, _ := signIn(t, email, wrongPassword)
		assert.Equal(t, http.StatusUnauthorized, status)
	}
}
