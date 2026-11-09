package broker

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	_topic   = "work"
	_retries = 2
	_wait    = 5 * time.Second
)

var errDown = errors.New("down")

func TestTheChainRetriesThenPoisons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		handle    func() error
		wantCalls int32
		poisoned  bool
	}{
		{"a success is handled once", func() error { return nil }, 1, false},
		{"a failure is retried, then poisoned", func() error { return errDown }, _retries + 1, true},
		{"a permanent failure is poisoned at once", func() error { return Permanent(errDown) }, 1, true},
		{"a panic is recovered and retried", func() error { panic("boom") }, _retries + 1, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			poison, done := run(t, func(context.Context, []byte) error {
				calls.Add(1)

				return tc.handle()
			})

			if tc.poisoned {
				select {
				case msg := <-poison:
					assert.Equal(t, "m-1", msg.UUID)
					assert.NotEmpty(t, msg.Metadata.Get("reason_poisoned"))
					msg.Ack()
				case <-time.After(_wait):
					t.Fatal("the message never reached the poison topic")
				}
			} else {
				require.Eventually(t, func() bool { return calls.Load() == 1 }, _wait, time.Millisecond)
			}

			done()
			assert.Equal(t, tc.wantCalls, calls.Load())
		})
	}
}

func TestPermanentWrapsTheError(t *testing.T) {
	t.Parallel()

	err := Permanent(errDown)

	assert.True(t, IsPermanent(err))
	require.ErrorIs(t, err, errDown)
	assert.False(t, IsPermanent(errDown))
}

func TestNewFailsWithoutAServer(t *testing.T) {
	t.Parallel()

	_, err := New("nats://127.0.0.1:1", Settings{AckWait: time.Second, ShutdownTimeout: time.Second},
		slog.New(slog.DiscardHandler))
	require.Error(t, err)
}

func run(t *testing.T, h HandlerFunc) (poison <-chan *message.Message, done func()) {
	t.Helper()

	logger := watermill.NopLogger{}
	pubsub := gochannel.NewGoChannel(gochannel.Config{}, logger)

	router, err := newRouter(pubsub, Settings{
		MaxRetries: _retries, RetryInitial: time.Millisecond, RetryMax: time.Millisecond, ShutdownTimeout: time.Second,
	}, logger, slog.New(slog.DiscardHandler))
	require.NoError(t, err)

	b := &Broker{pub: pubsub, sub: pubsub, router: router}
	b.Handle(_topic, h)

	poison, err = pubsub.Subscribe(t.Context(), PoisonTopic)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		assert.NoError(t, b.Run(ctx))
	}()

	<-router.Running()
	require.NoError(t, b.Publish(t.Context(), _topic, "m-1", []byte("{}")))

	return poison, func() {
		cancel()
		<-stopped
	}
}
