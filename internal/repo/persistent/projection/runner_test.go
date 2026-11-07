package projection_test

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"sync"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	_eventually = 2 * time.Second
	_tick       = 5 * time.Millisecond
	_poll       = 10 * time.Millisecond
	_never      = time.Hour
)

var (
	errHandle  = errors.New("handle failed")
	errNoTx    = errors.New("advance outside a transaction")
	errNoTable = errors.New("relation projection_checkpoints does not exist")
	errNoRow   = errors.New("no rows in result set")
)

type ticked struct{ N int }

func (ticked) EventType() string  { return "Ticked" }
func (ticked) SchemaVersion() int { return 1 }

type tocked struct{}

func (tocked) EventType() string  { return "Tocked" }
func (tocked) SchemaVersion() int { return 1 }

// fakeStore keeps events and checkpoints in memory. A transaction's checkpoint moves apply only
// when fn succeeds, as Postgres would commit them; a move made by another transaction meanwhile,
// such as a rebuild's reset, stays. It counts the reads that failed and the locks it refused, so a
// test can wait for the runner to have tried rather than sleep.
type fakeStore struct {
	mu          sync.Mutex
	events      []entity.RecordedEvent
	checkpoints map[string]int64
	locked      map[string]bool
	refused     map[string]int
	readErr     error
	failedReads int
	ensureFails int
	notify      chan struct{}
	txs         int
}

func newFakeStore(n int) *fakeStore {
	s := &fakeStore{
		checkpoints: map[string]int64{}, locked: map[string]bool{}, refused: map[string]int{},
		notify: make(chan struct{}, 1),
	}
	for range n {
		s.append()
	}

	return s
}

func (s *fakeStore) append() {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := int64(len(s.events) + 1)
	s.events = append(s.events, entity.RecordedEvent{Position: p, StreamID: "tick", Event: ticked{N: int(p)}})
}

func (s *fakeStore) checkpoint(name string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.checkpoints[name]
}

func (s *fakeStore) setCheckpoint(name string, p int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.checkpoints[name] = p
}

func (s *fakeStore) lock(name string, held bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.locked[name] = held
}

func (s *fakeStore) ReadAll(_ context.Context, after int64, limit int) ([]entity.RecordedEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.readErr != nil {
		s.failedReads++

		return nil, s.readErr
	}

	var out []entity.RecordedEvent

	for _, e := range s.events {
		if e.Position > after && len(out) < limit {
			out = append(out, e)
		}
	}

	return out, nil
}

type txKey struct{}

// fakeTx holds the checkpoint moves of one transaction until it commits.
type fakeTx struct {
	moves map[string]int64
}

func (s *fakeStore) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	s.mu.Lock()
	s.txs++
	s.mu.Unlock()

	tx := &fakeTx{moves: map[string]int64{}}

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	maps.Copy(s.checkpoints, tx.moves)

	return nil
}

func (s *fakeStore) EnsureCheckpoint(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ensureFails > 0 {
		s.ensureFails--

		return errNoTable
	}

	if _, ok := s.checkpoints[name]; !ok {
		s.checkpoints[name] = 0
	}

	return nil
}

func (s *fakeStore) Checkpoint(_ context.Context, name string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.checkpoints[name]
	if !ok {
		return 0, errNoRow
	}

	return p, nil
}

func (s *fakeStore) TryLock(_ context.Context, name string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.locked[name] {
		s.refused[name]++

		return false, nil
	}

	return true, nil
}

// refusedLock is true once the store has refused name's lock n times.
func (s *fakeStore) refusedLock(name string, n int) func() bool {
	return func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()

		return s.refused[name] >= n
	}
}

// failedToRead is true once n reads have failed.
func (s *fakeStore) failedToRead(n int) func() bool {
	return func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()

		return s.failedReads >= n
	}
}

func (s *fakeStore) AdvanceCheckpoint(ctx context.Context, name string, from, to int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.checkpoints[name] != from {
		return false, nil
	}

	tx, ok := ctx.Value(txKey{}).(*fakeTx)
	if !ok {
		return false, errNoTx
	}

	tx.moves[name] = to

	return true, nil
}

// Notifications passes on what the test sends on s.notify, until ctx ends.
func (s *fakeStore) Notifications(ctx context.Context) <-chan struct{} {
	out := make(chan struct{})

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.notify:
				select {
				case out <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out
}

// recorder is a subscriber that keeps the positions it handled: of Ticked events, or of the types
// in handles when set. failAt makes it fail once at that position; onHandle runs before it records.
type recorder struct {
	name     string
	handles  []entity.Event
	mu       sync.Mutex
	handled  []int64
	failAt   int64
	failed   bool
	onHandle func(e *entity.RecordedEvent)
}

func (r *recorder) Name() string { return r.name }

func (r *recorder) Handles() []entity.Event {
	if r.handles != nil {
		return r.handles
	}

	return []entity.Event{ticked{}}
}

func (r *recorder) Handle(_ context.Context, e *entity.RecordedEvent) error {
	if r.onHandle != nil {
		r.onHandle(e)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if e.Position == r.failAt && !r.failed {
		r.failed = true

		return errHandle
	}

	r.handled = append(r.handled, e.Position)

	return nil
}

// projector is a recorder with tables, so it can be rebuilt.
type projector struct{ recorder }

func (*projector) Tables() []string { return []string{"t"} }

func (r *recorder) positions() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]int64(nil), r.handled...)
}

func start(t *testing.T, s *fakeStore, poll time.Duration, subs ...*recorder) {
	t.Helper()

	runner := projection.New(slog.New(slog.DiscardHandler), s, poll, 2)
	for _, sub := range subs {
		runner.Add(sub)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- runner.Run(ctx) }()

	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
}

func handledUpTo(s *fakeStore, r *recorder, p int64) func() bool {
	return func() bool { return s.checkpoint(r.name) == p }
}

func TestARunnerHandsEverySubscriberEveryEventInOrder(t *testing.T) {
	t.Parallel()

	s := newFakeStore(5)
	a, b := &recorder{name: "a"}, &recorder{name: "b"}

	start(t, s, _poll, a, b)

	require.Eventually(t, handledUpTo(s, a, 5), _eventually, _tick)
	require.Eventually(t, handledUpTo(s, b, 5), _eventually, _tick)
	assert.Equal(t, []int64{1, 2, 3, 4, 5}, a.positions())
	assert.Equal(t, []int64{1, 2, 3, 4, 5}, b.positions())
}

func TestAFailedEventIsRetriedWithoutMovingTheCheckpoint(t *testing.T) {
	t.Parallel()

	s := newFakeStore(3)
	r := &recorder{name: "r", failAt: 2}

	start(t, s, _poll, r)

	require.Eventually(t, handledUpTo(s, r, 3), _eventually, _tick)
	assert.Equal(t, []int64{1, 2, 3}, r.positions())
}

func TestASubscriberLockedElsewhereIsLeftAlone(t *testing.T) {
	t.Parallel()

	s := newFakeStore(2)
	s.lock("r", true)

	r := &recorder{name: "r"}
	start(t, s, _poll, r)

	require.Eventually(t, s.refusedLock("r", 2), _eventually, _tick, "the runner tries again after a refusal")
	assert.Empty(t, r.positions())
	assert.Zero(t, s.checkpoint("r"))

	s.lock("r", false)
	require.Eventually(t, handledUpTo(s, r, 2), _eventually, _tick)
}

func TestAResetCheckpointMakesTheRunnerReplayFromTheStart(t *testing.T) {
	t.Parallel()

	s := newFakeStore(3)
	r := &recorder{name: "r"}

	var once sync.Once

	// A rebuild resets the checkpoint while the runner is handling the second event: that event's
	// transaction must not move it, and the runner must start over.
	r.onHandle = func(e *entity.RecordedEvent) {
		if e.Position == 2 {
			once.Do(func() { s.setCheckpoint("r", 0) })
		}
	}

	start(t, s, _poll, r)

	require.Eventually(t, handledUpTo(s, r, 3), _eventually, _tick)
	assert.Equal(t, []int64{1, 2, 1, 2, 3}, r.positions())
}

func TestANotificationWakesAnIdleRunner(t *testing.T) {
	t.Parallel()

	s := newFakeStore(1)
	r := &recorder{name: "r"}

	start(t, s, _never, r)
	require.Eventually(t, handledUpTo(s, r, 1), _eventually, _tick)

	s.append()

	s.notify <- struct{}{}

	require.Eventually(t, handledUpTo(s, r, 2), _eventually, _tick)
}

func TestAReadFailureIsRetried(t *testing.T) {
	t.Parallel()

	s := newFakeStore(1)
	s.readErr = errHandle

	r := &recorder{name: "r"}
	start(t, s, _poll, r)

	require.Eventually(t, s.failedToRead(2), _eventually, _tick, "the runner reads again after a failure")

	s.mu.Lock()
	s.readErr = nil
	s.mu.Unlock()

	require.Eventually(t, handledUpTo(s, r, 1), _eventually, _tick)
}

// Events a subscriber does not handle move its checkpoint without reaching it, one transaction per
// batch rather than per event.
func TestEventsASubscriberDoesNotHandlePassItBy(t *testing.T) {
	t.Parallel()

	s := newFakeStore(5)
	r := &recorder{name: "r", handles: []entity.Event{tocked{}}}

	start(t, s, _poll, r)

	require.Eventually(t, handledUpTo(s, r, 5), _eventually, _tick)
	assert.Empty(t, r.positions())

	s.mu.Lock()
	defer s.mu.Unlock()

	assert.LessOrEqual(t, s.txs, 3, "five events in batches of two: one move per batch")
}

func TestRebuildRefusesAnUnknownName(t *testing.T) {
	t.Parallel()

	err := projection.Rebuild(context.Background(), nil, "missing", []projection.Projector{&projector{recorder{name: "tasks"}}}, nil)

	require.ErrorIs(t, err, projection.ErrUnknownProjector)
	assert.Contains(t, err.Error(), "tasks", "the error names the projectors that can be rebuilt")
}

// The worker can start before the app has applied the schema: the runner keeps trying to create
// its checkpoint, and reads only once it has one.
func TestARunnerRetriesItsCheckpointUntilTheSchemaExists(t *testing.T) {
	t.Parallel()

	s := newFakeStore(2)
	s.ensureFails = 3

	r := &recorder{name: "r"}
	start(t, s, _poll, r)

	require.Eventually(t, handledUpTo(s, r, 2), _eventually, _tick)
	assert.Equal(t, []int64{1, 2}, r.positions())
}
