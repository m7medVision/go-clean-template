package notify_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/notify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _loginURL = "https://example.com/login"

var errQueueDown = errors.New("broker down")

type fakePublisher struct {
	err   error
	topic string
	id    string
	body  []byte
}

func (f *fakePublisher) Publish(_ context.Context, topic, id string, body []byte) error {
	f.topic, f.id, f.body = topic, id, body

	return f.err
}

func TestWelcomePublishesTheWelcomeEmail(t *testing.T) {
	t.Parallel()

	q := &fakePublisher{}

	err := notify.New(q, _loginURL).Welcome(t.Context(), &entity.User{ID: "u-1", Username: "testuser", Email: "test@example.com"})
	require.NoError(t, err)

	assert.Equal(t, entity.EmailQueue, q.topic)
	assert.Equal(t, "welcome-u-1", q.id)

	email, err := entity.DecodeEmail(q.body)
	require.NoError(t, err)
	assert.Equal(t, entity.WelcomeEmail{
		To:       "test@example.com",
		Username: "testuser",
		LoginURL: _loginURL,
	}, email)
}

func TestWelcomeReturnsThePublishError(t *testing.T) {
	t.Parallel()

	q := &fakePublisher{err: errQueueDown}

	err := notify.New(q, _loginURL).Welcome(t.Context(), &entity.User{Username: "testuser", Email: "test@example.com"})

	require.ErrorIs(t, err, errQueueDown)
}
