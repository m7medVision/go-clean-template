// Package queue implements the Postgres-backed message queue: one queue_messages table shared by
// every queue name.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/repo/persistent/queries"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/evrone/go-clean-template/pkg/worker"
)

// The worker binary consumes this repository directly.
var _ worker.Queue = repo.QueueRepo(nil)

// Repo -.
type Repo struct {
	q *queries.Queries
}

// New returns a Queue repository instrumented with OpenTelemetry tracing spans.
func New(pg *postgres.Postgres) repo.QueueRepo {
	return newTraced(&Repo{queries.New(pg.Pool)})
}

// Send enqueues body on queue and returns the message id.
func (r *Repo) Send(ctx context.Context, queue string, body json.RawMessage) (int64, error) {
	id, err := r.q.SendQueueMessage(ctx, queries.SendQueueMessageParams{Queue: queue, Body: body})
	if err != nil {
		return 0, fmt.Errorf("QueueRepo - Send - r.q.SendQueueMessage: %w", err)
	}

	return id, nil
}

// Read claims up to qty visible messages and hides them from other readers for vt.
func (r *Repo) Read(ctx context.Context, queue string, vt time.Duration, qty int) ([]worker.Message, error) {
	rows, err := r.q.ReadQueueMessages(ctx, queries.ReadQueueMessagesParams{
		Queue:     queue,
		Qty:       int32(min(qty, math.MaxInt32)), //nolint:gosec // clamped to the int32 range
		VtSeconds: vt.Seconds(),
	})
	if err != nil {
		return nil, fmt.Errorf("QueueRepo - Read - r.q.ReadQueueMessages: %w", err)
	}

	msgs := make([]worker.Message, len(rows))
	for i, row := range rows {
		msgs[i] = worker.Message(row)
	}

	return msgs, nil
}

// Delete removes a message for good. A message that is already gone is not an error: with
// at-least-once delivery another worker may have finished it after its visibility timeout.
func (r *Repo) Delete(ctx context.Context, _ string, id int64) error {
	if err := r.q.DeleteQueueMessage(ctx, id); err != nil {
		return fmt.Errorf("QueueRepo - Delete - r.q.DeleteQueueMessage: %w", err)
	}

	return nil
}
