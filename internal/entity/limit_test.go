package entity_test

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTooManyAttemptsCarriesTheWaitAndItsSentinel(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("log in: %w", entity.TooManyAttempts(90*time.Second))

	require.ErrorIs(t, err, entity.ErrTooManyAttempts)

	var retry *entity.RetryAfterError

	require.ErrorAs(t, err, &retry)
	assert.Equal(t, 90*time.Second, retry.RetryAfter)
	assert.Equal(t, entity.KindRateLimited, entity.KindOf(err))
	assert.Equal(t, "too many attempts", entity.Message(err))
	assert.EqualError(t, err, "log in: too many attempts")
}

func TestRateLimitedIsNotLogged(t *testing.T) {
	t.Parallel()

	level, logged := entity.KindRateLimited.LogLevel()

	assert.False(t, logged)
	assert.Equal(t, slog.Level(0), level)
}

func TestTheSentinelAloneCarriesNoWait(t *testing.T) {
	t.Parallel()

	var retry *entity.RetryAfterError

	assert.NotErrorAs(t, fmt.Errorf("op: %w", entity.ErrTooManyAttempts), &retry)
}
