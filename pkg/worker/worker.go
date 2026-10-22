// Package worker polls queues and dispatches each message to a registered handler.
//
// Delivery is at-least-once: a message is deleted only after its handler returns nil. A message
// whose handler fails stays in the queue and is redelivered when its visibility timeout expires,
// so handlers must be idempotent.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/m7medvision/go-clean-template/pkg/logger"
)

const (
	_defaultPollInterval      = time.Second
	_defaultVisibilityTimeout = 30 * time.Second
	_defaultBatchSize         = 10
	_defaultShutdownTimeout   = 10 * time.Second
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

// HandlerFunc processes one message. Returning nil deletes the message; returning an error leaves
// it for redelivery.
type HandlerFunc func(ctx context.Context, msg *Message) error

// Worker -.
type Worker struct {
	queue    Queue
	handlers map[string]HandlerFunc
	wg       sync.WaitGroup

	// pollCtx stops the read loops; handlerCtx is canceled only once ShutdownTimeout expires, so
	// in-flight handlers get a chance to finish.
	pollCtx      context.Context
	stopPolling  context.CancelFunc
	handlerCtx   context.Context
	stopHandlers context.CancelFunc

	pollInterval      time.Duration
	visibilityTimeout time.Duration
	batchSize         int
	shutdownTimeout   time.Duration

	logger logger.Interface
}

// New -.
func New(l logger.Interface, q Queue, opts ...Option) *Worker {
	pollCtx, stopPolling := context.WithCancel(context.Background())
	handlerCtx, stopHandlers := context.WithCancel(context.Background())

	w := &Worker{
		queue:             q,
		handlers:          make(map[string]HandlerFunc),
		pollCtx:           pollCtx,
		stopPolling:       stopPolling,
		handlerCtx:        handlerCtx,
		stopHandlers:      stopHandlers,
		pollInterval:      _defaultPollInterval,
		visibilityTimeout: _defaultVisibilityTimeout,
		batchSize:         _defaultBatchSize,
		shutdownTimeout:   _defaultShutdownTimeout,
		logger:            l,
	}

	// Custom options
	for _, opt := range opts {
		opt(w)
	}

	return w
}

// Handle registers the handler for a queue. Call it before Start.
func (w *Worker) Handle(queue string, h HandlerFunc) {
	w.handlers[queue] = h
}

// Queues lists the queues that have a handler, sorted.
func (w *Worker) Queues() []string {
	return slices.Sorted(maps.Keys(w.handlers))
}

// Start starts one poll loop per registered queue.
func (w *Worker) Start() {
	for queue, h := range w.handlers {
		w.wg.Go(func() { w.poll(queue, h) })
	}

	w.logger.Info("worker - Worker - Started")
}

// Shutdown stops polling and waits for in-flight handlers, canceling them after ShutdownTimeout.
func (w *Worker) Shutdown() error {
	w.stopPolling()

	done := make(chan struct{})

	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(w.shutdownTimeout):
		w.logger.Warn("worker - Worker - Shutdown - timeout, canceling in-flight handlers")
		w.stopHandlers()
		<-done
	}

	w.stopHandlers()
	w.logger.Info("worker - Worker - Shutdown")

	return nil
}

func (w *Worker) poll(queue string, h HandlerFunc) {
	for w.pollCtx.Err() == nil {
		if w.readBatch(queue, h) > 0 {
			continue
		}

		select {
		case <-w.pollCtx.Done():
		case <-time.After(w.pollInterval):
		}
	}
}

// readBatch handles one batch and returns how many messages it read.
func (w *Worker) readBatch(queue string, h HandlerFunc) int {
	msgs, err := w.queue.Read(w.pollCtx, queue, w.visibilityTimeout, w.batchSize)
	if err != nil {
		if w.pollCtx.Err() == nil {
			w.logger.Error(fmt.Errorf("worker - readBatch - w.queue.Read %q: %w", queue, err))
		}

		return 0
	}

	for i := range msgs {
		// Messages left in the batch become visible again once their timeout expires.
		if w.pollCtx.Err() != nil {
			break
		}

		w.process(queue, h, &msgs[i])
	}

	return len(msgs)
}

func (w *Worker) process(queue string, h HandlerFunc, msg *Message) {
	if err := h(w.handlerCtx, msg); err != nil {
		w.logger.Error(fmt.Errorf("worker - process - queue %q, message %d: %w", queue, msg.ID, err))

		return
	}

	if err := w.queue.Delete(w.handlerCtx, queue, msg.ID); err != nil {
		w.logger.Error(fmt.Errorf("worker - process - w.queue.Delete %q, message %d: %w", queue, msg.ID, err))
	}
}
