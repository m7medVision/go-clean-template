// Package projection runs the event store's subscribers, projectors and event handlers, in the
// worker. Each subscriber reads the store in position order after its own checkpoint, and each event
// it handles commits in one transaction with the checkpoint's move, so a subscriber sees every
// event of the types it handles exactly once even across restarts, failures and several worker
// replicas. Events of other types move its checkpoint, a batch of them in one transaction.
package projection

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
)

var (
	// errMoved is a checkpoint that was not where the runner read it: a rebuild reset it, or
	// another runner moved it. The event's transaction rolls back and the runner reloads.
	errMoved = errors.New("projection: checkpoint moved")
	// errBusy is a subscriber another runner is handling right now.
	errBusy = errors.New("projection: subscriber busy")
)

// Store is what the runner reads and writes through.
type Store interface {
	ReadAll(ctx context.Context, after int64, limit int) ([]entity.RecordedEvent, error)
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
	EnsureCheckpoint(ctx context.Context, name string) error
	Checkpoint(ctx context.Context, name string) (int64, error)
	// TryLock takes name's lock until the context's transaction ends; false when another runner
	// holds it.
	TryLock(ctx context.Context, name string) (bool, error)
	// AdvanceCheckpoint moves name from `from` to `to` in the context's transaction; false when it
	// was not at from.
	AdvanceCheckpoint(ctx context.Context, name string, from, to int64) (bool, error)
	// Notifications receives a value when events may have been appended. Nil polls only.
	Notifications() <-chan struct{}
}

// Runner runs subscribers until Shutdown.
type Runner struct {
	store        Store
	subscribers  []repo.Subscriber
	pollInterval time.Duration
	batchSize    int
	logger       *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	wakeMu sync.Mutex
	wakes  []chan struct{}
}

// New returns a runner reading s, at most batchSize events at once. An idle subscriber waits
// pollInterval for a notification before it reads again.
func New(l *slog.Logger, s Store, pollInterval time.Duration, batchSize int) *Runner {
	ctx, cancel := context.WithCancel(context.Background())

	return &Runner{
		store:        s,
		pollInterval: pollInterval,
		batchSize:    batchSize,
		logger:       l,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Add registers a subscriber. Call it before Start.
func (r *Runner) Add(s repo.Subscriber) {
	r.subscribers = append(r.subscribers, s)
}

// Start runs one loop per subscriber, and one that fans the store's notifications out to them.
func (r *Runner) Start() {
	for _, s := range r.subscribers {
		wake := make(chan struct{}, 1)

		r.wakeMu.Lock()
		r.wakes = append(r.wakes, wake)
		r.wakeMu.Unlock()

		r.wg.Go(func() { r.run(s, wake) })
	}

	if n := r.store.Notifications(); n != nil {
		r.wg.Go(func() { r.fanOut(n) })
	}

	r.logger.Info("projection - runner started", "subscribers", len(r.subscribers))
}

// Shutdown stops every loop and waits for the event in hand to finish or roll back.
func (r *Runner) Shutdown() {
	r.cancel()
	r.wg.Wait()
	r.logger.Info("projection - Runner - Shutdown")
}

func (r *Runner) fanOut(n <-chan struct{}) {
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-n:
			r.wakeMu.Lock()
			for _, w := range r.wakes {
				select {
				case w <- struct{}{}:
				default:
				}
			}
			r.wakeMu.Unlock()
		}
	}
}

func (r *Runner) run(s repo.Subscriber, wake <-chan struct{}) {
	if !r.ensureCheckpoint(s) {
		return
	}

	handles := make(map[string]bool, len(s.Handles()))
	for _, e := range s.Handles() {
		handles[e.EventType()] = true
	}

	for r.ctx.Err() == nil {
		full, err := r.batch(s, handles)

		switch {
		case full, errors.Is(err, errMoved):
			// More to read, or a reset to reload: go again at once.
			continue
		case err != nil && !errors.Is(err, errBusy):
			r.logError(s, err)
		}

		select {
		case <-r.ctx.Done():
		case <-wake:
		case <-time.After(r.pollInterval):
		}
	}
}

// ensureCheckpoint creates s's checkpoint, trying again every poll interval until it can: the
// worker can start before the app has applied the schema. It is false when the runner stops first.
func (r *Runner) ensureCheckpoint(s repo.Subscriber) bool {
	for r.ctx.Err() == nil {
		err := r.store.EnsureCheckpoint(r.ctx, s.Name())
		if err == nil {
			return true
		}

		r.logError(s, fmt.Errorf("EnsureCheckpoint: %w", err))

		select {
		case <-r.ctx.Done():
		case <-time.After(r.pollInterval):
		}
	}

	return false
}

func (r *Runner) logError(s repo.Subscriber, err error) {
	if r.ctx.Err() == nil {
		r.logger.Error("projection - handle an event", "subscriber", s.Name(), "error", err)
	}
}

// batch handles the events after s's checkpoint, at most a batch of them. Each event of a type s
// handles commits with the checkpoint's move to it, past any events before it s does not handle;
// events s does not handle at the batch's end move the checkpoint in one transaction of their own.
// It reports whether the batch was full, so the loop reads again without waiting.
func (r *Runner) batch(s repo.Subscriber, handles map[string]bool) (bool, error) {
	checkpoint, err := r.store.Checkpoint(r.ctx, s.Name())
	if err != nil {
		return false, fmt.Errorf("Checkpoint: %w", err)
	}

	events, err := r.store.ReadAll(r.ctx, checkpoint, r.batchSize)
	if err != nil {
		return false, fmt.Errorf("ReadAll: %w", err)
	}

	for i := range events {
		e := &events[i]
		if !handles[e.Event.EventType()] {
			continue
		}

		err = r.move(s, checkpoint, e.Position, func(ctx context.Context) error {
			if err := s.Handle(ctx, e); err != nil {
				return fmt.Errorf("handle %s at %d: %w", e.Event.EventType(), e.Position, err)
			}

			return nil
		})
		if err != nil {
			return false, err
		}

		checkpoint = e.Position
	}

	if n := len(events); n > 0 && checkpoint != events[n-1].Position {
		if err := r.move(s, checkpoint, events[n-1].Position, nil); err != nil {
			return false, err
		}
	}

	return len(events) == r.batchSize, nil
}

// move runs handle, when there is one, and moves s's checkpoint from `from` to `to`, in one
// transaction.
func (r *Runner) move(s repo.Subscriber, from, to int64, handle func(ctx context.Context) error) error {
	return r.store.InTx(r.ctx, func(ctx context.Context) error {
		locked, err := r.store.TryLock(ctx, s.Name())
		if err != nil {
			return fmt.Errorf("TryLock: %w", err)
		}

		if !locked {
			return errBusy
		}

		if handle != nil {
			if err := handle(ctx); err != nil {
				return err
			}
		}

		moved, err := r.store.AdvanceCheckpoint(ctx, s.Name(), from, to)
		if err != nil {
			return fmt.Errorf("AdvanceCheckpoint: %w", err)
		}

		if !moved {
			return errMoved
		}

		return nil
	})
}
