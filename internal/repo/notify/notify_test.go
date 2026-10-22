package notify_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/notify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _loginURL = "https://example.com/login"

var errQueueDown = errors.New("queue down")

type fakeQueue struct {
	repo.QueueRepo

	err   error
	queue string
	body  json.RawMessage
}

func (f *fakeQueue) Send(_ context.Context, queue string, body json.RawMessage) (int64, error) {
	f.queue, f.body = queue, body

	return 1, f.err
}

func TestWelcomeEnqueuesTheWelcomeEmail(t *testing.T) {
	t.Parallel()

	q := &fakeQueue{}

	err := notify.New(q, _loginURL).Welcome(t.Context(), &entity.User{Username: "testuser", Email: "test@example.com"})
	require.NoError(t, err)

	assert.Equal(t, entity.EmailQueue, q.queue)

	email, err := entity.DecodeEmail(q.body)
	require.NoError(t, err)
	assert.Equal(t, entity.WelcomeEmail{
		To:       "test@example.com",
		Username: "testuser",
		LoginURL: _loginURL,
	}, email)
}

func TestWelcomeReturnsTheQueueError(t *testing.T) {
	t.Parallel()

	q := &fakeQueue{err: errQueueDown}

	err := notify.New(q, _loginURL).Welcome(t.Context(), &entity.User{Username: "testuser", Email: "test@example.com"})

	require.ErrorIs(t, err, errQueueDown)
}
