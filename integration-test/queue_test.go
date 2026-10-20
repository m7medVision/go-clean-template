package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queue"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// exampleQueue is consumed by the worker service; visibilityQueue is read by nobody but the test.
	exampleQueue    = "example"
	visibilityQueue = "integration-test-visibility"

	queueWait = 30 * time.Second
	queuePoll = 200 * time.Millisecond
	queueVT   = time.Second
	queueQty  = 10
)

func newQueuePostgres(t *testing.T) *postgres.Postgres {
	t.Helper()

	// Set by docker-compose-integration-test.yml: Postgres as seen from inside the compose network.
	pgURL := os.Getenv("PG_URL")
	require.NotEmpty(t, pgURL, "PG_URL is not set")

	pg, err := postgres.New(pgURL)
	require.NoError(t, err)
	t.Cleanup(pg.Close)

	return pg
}

func queueRowExists(ctx context.Context, t *testing.T, pg *postgres.Postgres, id int64) bool {
	t.Helper()

	var exists bool

	err := pg.Pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM queue_messages WHERE id = $1)", id).Scan(&exists)
	require.NoError(t, err)

	return exists
}

// The worker service reads the example queue and deletes each message its handler accepts.
func TestWorkerConsumesExampleQueue(t *testing.T) {
	ctx := context.Background()
	pg := newQueuePostgres(t)

	id, err := queue.New(pg).Send(ctx, exampleQueue, json.RawMessage(`{"text":"`+t.Name()+`"}`))
	require.NoError(t, err)

	require.Eventually(t, func() bool { return !queueRowExists(ctx, t, pg, id) },
		queueWait, queuePoll, "worker never consumed message %d", id)
}

// A read message is hidden for the visibility timeout, then redelivered with a higher read count.
func TestQueueVisibilityTimeoutRedelivers(t *testing.T) {
	ctx := context.Background()
	pg := newQueuePostgres(t)
	q := queue.New(pg)

	id, err := q.Send(ctx, visibilityQueue, json.RawMessage(`{"n":1}`))
	require.NoError(t, err)

	first, err := q.Read(ctx, visibilityQueue, queueVT, queueQty)
	require.NoError(t, err)
	require.Len(t, first, 1)
	assert.Equal(t, id, first[0].ID)
	assert.Equal(t, int32(1), first[0].ReadCount)
	assert.JSONEq(t, `{"n":1}`, string(first[0].Body))

	hidden, err := q.Read(ctx, visibilityQueue, queueVT, queueQty)
	require.NoError(t, err)
	assert.Empty(t, hidden, "message must stay hidden until its visibility timeout expires")

	var again []int64

	require.Eventually(t, func() bool {
		msgs, err := q.Read(ctx, visibilityQueue, queueVT, queueQty)
		require.NoError(t, err)

		for _, m := range msgs {
			if m.ID == id {
				assert.Equal(t, int32(2), m.ReadCount)

				again = append(again, m.ID)
			}
		}

		return len(again) == 1
	}, queueWait, queuePoll, "message %d was never redelivered", id)

	require.NoError(t, q.Delete(ctx, visibilityQueue, id))
	assert.False(t, queueRowExists(ctx, t, pg, id))
}
