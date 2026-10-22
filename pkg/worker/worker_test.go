package worker_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	_queue   = "jobs"
	_wait    = 2 * time.Second
	_tick    = 5 * time.Millisecond
	_poll    = 10 * time.Millisecond
	_timeout = time.Second
)

var (
	errHandler = errors.New("handler failed")
	errRead    = errors.New("read failed")
)

// fakeQueue hands out each queued message once and records what happened to it.
type fakeQueue struct {
	mu       sync.Mutex
	pending  map[string][]worker.Message
	deleted  []int64
	readErrs int
}

func newFakeQueue(msgs ...worker.Message) *fakeQueue {
	return &fakeQueue{pending: map[string][]worker.Message{_queue: msgs}}
}

func (f *fakeQueue) Read(_ context.Context, queue string, _ time.Duration, qty int) ([]worker.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.readErrs > 0 {
		f.readErrs--

		return nil, errRead
	}

	n := min(qty, len(f.pending[queue]))
	msgs := f.pending[queue][:n]
	f.pending[queue] = f.pending[queue][n:]

	return msgs, nil
}

func (f *fakeQueue) Delete(_ context.Context, _ string, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deleted = append(f.deleted, id)

	return nil
}

func (f *fakeQueue) deletedIDs() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.deleted)
}

func newWorker(q worker.Queue) *worker.Worker {
	return worker.New(
		logger.New("error"),
		q,
		worker.PollInterval(_poll),
		worker.BatchSize(2),
		worker.ShutdownTimeout(_timeout),
	)
}

func TestWorker_DeletesMessageWhenHandlerSucceeds(t *testing.T) {
	t.Parallel()

	q := newFakeQueue(worker.Message{ID: 1}, worker.Message{ID: 2}, worker.Message{ID: 3})
	w := newWorker(q)
	w.Handle(_queue, func(context.Context, *worker.Message) error { return nil })

	w.Start()

	require.Eventually(t, func() bool { return len(q.deletedIDs()) == 3 }, _wait, _tick)
	require.NoError(t, w.Shutdown())
	assert.Equal(t, []int64{1, 2, 3}, q.deletedIDs())
}

func TestWorker_KeepsMessageWhenHandlerFails(t *testing.T) {
	t.Parallel()

	q := newFakeQueue(worker.Message{ID: 1})
	handled := make(chan struct{})

	w := newWorker(q)
	w.Handle(_queue, func(context.Context, *worker.Message) error {
		close(handled)

		return errHandler
	})

	w.Start()
	<-handled
	require.NoError(t, w.Shutdown())

	assert.Empty(t, q.deletedIDs())
}

func TestWorker_KeepsPollingAfterReadError(t *testing.T) {
	t.Parallel()

	q := newFakeQueue(worker.Message{ID: 7})
	q.readErrs = 2

	w := newWorker(q)
	w.Handle(_queue, func(context.Context, *worker.Message) error { return nil })

	w.Start()

	require.Eventually(t, func() bool { return len(q.deletedIDs()) == 1 }, _wait, _tick)
	require.NoError(t, w.Shutdown())
}

func TestWorker_DispatchesEachQueueToItsOwnHandler(t *testing.T) {
	t.Parallel()

	q := &fakeQueue{pending: map[string][]worker.Message{
		"a": {{ID: 1, Queue: "a"}},
		"b": {{ID: 2, Queue: "b"}},
	}}

	var (
		mu  sync.Mutex
		got = map[string]int64{}
	)

	record := func(queue string) worker.HandlerFunc {
		return func(_ context.Context, msg *worker.Message) error {
			mu.Lock()
			defer mu.Unlock()

			got[queue] = msg.ID

			return nil
		}
	}

	w := newWorker(q)
	w.Handle("a", record("a"))
	w.Handle("b", record("b"))

	w.Start()

	require.Eventually(t, func() bool { return len(q.deletedIDs()) == 2 }, _wait, _tick)
	require.NoError(t, w.Shutdown())

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, map[string]int64{"a": 1, "b": 2}, got)
}

func TestWorker_ShutdownWaitsForInFlightHandler(t *testing.T) {
	t.Parallel()

	q := newFakeQueue(worker.Message{ID: 1})
	started := make(chan struct{})
	release := make(chan struct{})

	w := newWorker(q)
	w.Handle(_queue, func(context.Context, *worker.Message) error {
		close(started)
		<-release

		return nil
	})

	w.Start()
	<-started

	done := make(chan error)

	go func() { done <- w.Shutdown() }()

	select {
	case <-done:
		t.Fatal("Shutdown returned while a handler was still running")
	case <-time.After(5 * _poll):
	}

	close(release)

	require.NoError(t, <-done)
	assert.Equal(t, []int64{1}, q.deletedIDs())
}

func TestWorker_ShutdownCancelsHandlerAfterTimeout(t *testing.T) {
	t.Parallel()

	q := newFakeQueue(worker.Message{ID: 1})
	started := make(chan struct{})

	w := worker.New(logger.New("error"), q, worker.PollInterval(_poll), worker.ShutdownTimeout(_poll))
	w.Handle(_queue, func(ctx context.Context, _ *worker.Message) error {
		close(started)
		<-ctx.Done()

		return ctx.Err()
	})

	w.Start()
	<-started

	require.NoError(t, w.Shutdown())
	assert.Empty(t, q.deletedIDs())
}

func TestWorker_QueuesListsHandledQueuesSorted(t *testing.T) {
	t.Parallel()

	w := newWorker(newFakeQueue())
	noop := func(context.Context, *worker.Message) error { return nil }

	w.Handle("mail", noop)
	w.Handle("example", noop)

	assert.Equal(t, []string{"example", "mail"}, w.Queues())
}
