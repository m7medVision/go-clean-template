// Package worker polls queues and dispatches each message to a registered handler.
//
// Delivery is at-least-once: a message is deleted only after its handler returns nil. A message
// whose handler fails stays in the queue and is redelivered when its visibility timeout expires,
// so handlers must be idempotent.
package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// Message is a message read from a queue.
type Message struct {
	ID         int64
	Queue      string
	Body       json.RawMessage
	ReadCount  int32
	EnqueuedAt time.Time
	VisibleAt  time.Time
}

// Queue is what the worker reads from. Read hides the returned messages for vt; Delete removes one
// for good.
type Queue interface {
	Read(ctx context.Context, queue string, vt time.Duration, qty int) ([]Message, error)
	Delete(ctx context.Context, queue string, id int64) error
}

// Settings say how the worker reads its queues.
type Settings struct {
	// PollInterval is how long a queue loop sleeps after a read returns no messages.
	PollInterval time.Duration
	// VisibilityTimeout is how long a read message stays hidden from other readers. A message whose
	// handler fails is redelivered once it expires.
	VisibilityTimeout time.Duration
	// BatchSize is the most messages read per poll.
	BatchSize int
	// ShutdownTimeout is how long Run waits, once its context ends, for the handlers in flight
	// before it cancels theirs.
	ShutdownTimeout time.Duration
}

// HandlerFunc processes one message. Returning nil deletes the message; returning an error leaves
// it for redelivery.
type HandlerFunc func(ctx context.Context, msg *Message) error

// Worker reads its queues and hands each message to the queue's handler.
type Worker struct {
	queue    Queue
	handlers map[string]HandlerFunc
	settings Settings
	logger   *slog.Logger
}

// New returns a worker reading q with s.
func New(l *slog.Logger, q Queue, s Settings) *Worker {
	return &Worker{queue: q, handlers: make(map[string]HandlerFunc), settings: s, logger: l}
}

// Handle registers the handler for a queue. Call it before Run.
func (w *Worker) Handle(queue string, h HandlerFunc) {
	w.handlers[queue] = h
}

// Run polls every queue that has a handler until ctx ends. Then it waits for the handlers in
// flight, and cancels their context once ShutdownTimeout passes. It returns nil: a failed read or
// handler is logged and retried, never fatal.
func (w *Worker) Run(ctx context.Context) error {
	// Handlers keep ctx's values but not its end, so they get ShutdownTimeout to finish.
	handlerCtx, stopHandlers := context.WithCancel(context.WithoutCancel(ctx))
	defer stopHandlers()

	var wg sync.WaitGroup

	for queue, h := range w.handlers {
		wg.Go(func() { w.poll(ctx, handlerCtx, queue, h) })
	}

	w.logger.Info("worker - started", "queues", len(w.handlers))

	<-ctx.Done()

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(w.settings.ShutdownTimeout):
		w.logger.Warn("worker - shutdown timeout, canceling in-flight handlers")
		stopHandlers()
		<-done
	}

	w.logger.Info("worker - stopped")

	return nil
}

// poll reads queue until ctx ends, and hands each message to h under handlerCtx.
func (w *Worker) poll(ctx, handlerCtx context.Context, queue string, h HandlerFunc) {
	for ctx.Err() == nil {
		if w.readBatch(ctx, handlerCtx, queue, h) > 0 {
			continue
		}

		select {
		case <-ctx.Done():
		case <-time.After(w.settings.PollInterval):
		}
	}
}

// readBatch handles one batch and returns how many messages it read.
func (w *Worker) readBatch(ctx, handlerCtx context.Context, queue string, h HandlerFunc) int {
	msgs, err := w.queue.Read(ctx, queue, w.settings.VisibilityTimeout, w.settings.BatchSize)
	if err != nil {
		if ctx.Err() == nil {
			w.logger.Error("worker - read a batch", "queue", queue, "error", err)
		}

		return 0
	}

	for i := range msgs {
		// Messages left in the batch become visible again once their timeout expires.
		if ctx.Err() != nil {
			break
		}

		w.process(handlerCtx, queue, h, &msgs[i])
	}

	return len(msgs)
}

func (w *Worker) process(ctx context.Context, queue string, h HandlerFunc, msg *Message) {
	if err := h(ctx, msg); err != nil {
		w.logger.Error("worker - handle a message", "queue", queue, "message", msg.ID, "error", err)

		return
	}

	if err := w.queue.Delete(ctx, queue, msg.ID); err != nil {
		w.logger.Error("worker - delete a message", "queue", queue, "message", msg.ID, "error", err)
	}
}
