// Package queue implements the Postgres-backed message queue: one queue_messages table shared by
// every queue name.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/m7medvision/go-clean-template/pkg/worker"
)

// The worker binary consumes the queue directly; notify enqueues through its own Queue port.
var _ worker.Queue = (*Repo)(nil)

// Repo is the queue. Its queries join the transaction the context carries.
type Repo struct {
	pg *postgres.Postgres
}

func New(pg *postgres.Postgres) *Repo {
	return &Repo{pg: pg}
}

func (r *Repo) q(ctx context.Context) *queries.Queries {
	return queries.New(r.pg.Executor(ctx))
}

// Send enqueues body on queue and returns the message id. It joins the transaction the context
// carries, so an event handler's message commits with its checkpoint, exactly once.
func (r *Repo) Send(ctx context.Context, queue string, body json.RawMessage) (int64, error) {
	id, err := r.q(ctx).SendQueueMessage(ctx, queries.SendQueueMessageParams{Queue: queue, Body: body})
	if err != nil {
		return 0, fmt.Errorf("queue - Send - r.q(ctx).SendQueueMessage: %w", err)
	}

	return id, nil
}

// Read claims up to qty visible messages and hides them from other readers for vt.
func (r *Repo) Read(ctx context.Context, queue string, vt time.Duration, qty int) ([]worker.Message, error) {
	rows, err := r.q(ctx).ReadQueueMessages(ctx, queries.ReadQueueMessagesParams{
		Queue:     queue,
		Qty:       int32(min(qty, math.MaxInt32)), //nolint:gosec // clamped to the int32 range
		VtSeconds: vt.Seconds(),
	})
	if err != nil {
		return nil, fmt.Errorf("queue - Read - r.q(ctx).ReadQueueMessages: %w", err)
	}

	msgs := make([]worker.Message, len(rows))
	for i := range rows {
		row := &rows[i]
		msgs[i] = worker.Message{
			ID:         row.ID,
			Queue:      row.Queue,
			Body:       row.Body,
			ReadCount:  row.ReadCount,
			EnqueuedAt: row.EnqueuedAt,
			VisibleAt:  row.VisibleAt,
		}
	}

	return msgs, nil
}

// Delete removes a message of queue for good. A message that is already gone, or is on another
// queue, is not an error: with at-least-once delivery another worker may have finished it after its
// visibility timeout.
func (r *Repo) Delete(ctx context.Context, queue string, id int64) error {
	if err := r.q(ctx).DeleteQueueMessage(ctx, queries.DeleteQueueMessageParams{ID: id, Queue: queue}); err != nil {
		return fmt.Errorf("queue - Delete - r.q(ctx).DeleteQueueMessage: %w", err)
	}

	return nil
}
