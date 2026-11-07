package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/eventcodec"
	"github.com/m7medvision/go-clean-template/internal/repo/eventstoretest"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	storePoolSize = 10
	// lateWriteWait is how long the ordering test lets a second writer run before it reads.
	lateWriteWait = 300 * time.Millisecond
)

var errRollBack = errors.New("roll back")

// newPostgres connects to the stack's database with room for concurrent transactions.
func newPostgres(t *testing.T) *postgres.Postgres {
	t.Helper()

	pgURL := os.Getenv("PG_URL")
	require.NotEmpty(t, pgURL, "PG_URL is not set")

	pg, err := postgres.New(pgURL, postgres.MaxPoolSize(storePoolSize))
	require.NoError(t, err)
	t.Cleanup(pg.Close)

	return pg
}

func newSuiteStore(pg *postgres.Postgres) repo.EventStore {
	return eventstore.New(pg, eventcodec.New(eventstoretest.Events()), eventstoretest.SnapshotEvery)
}

func TestPostgresEventStorePassesTheEventStoreSuite(t *testing.T) {
	pg := newPostgres(t)

	eventstoretest.Run(t, func(*testing.T) repo.EventStore { return newSuiteStore(pg) })
}

func TestSaveJoinsTheTransactionTheContextCarries(t *testing.T) {
	ctx := context.Background()
	pg := newPostgres(t)
	store := newSuiteStore(pg)
	n := eventstoretest.NewNote(uuid.NewString(), "never kept")

	err := pg.InTx(ctx, func(ctx context.Context) error {
		_, err := store.Save(ctx, n)
		require.NoError(t, err)

		return errRollBack
	})

	require.ErrorIs(t, err, errRollBack)
	require.ErrorIs(t, store.Load(ctx, n.StreamID(), &eventstoretest.Note{}), entity.ErrStreamNotFound)
}

func TestSaveStampsTheContextsMetadata(t *testing.T) {
	ctx := entity.ContextWithMetadata(context.Background(), entity.Metadata{UserID: "u-1", CausationID: "command Test"})
	pg := newPostgres(t)
	n := eventstoretest.NewNote(uuid.NewString(), "stamped")

	_, err := newSuiteStore(pg).Save(ctx, n)
	require.NoError(t, err)

	var raw json.RawMessage

	err = pg.Executor(ctx).QueryRow(ctx, "SELECT metadata FROM events WHERE stream_id = $1", n.StreamID()).Scan(&raw)
	require.NoError(t, err)

	var md entity.Metadata
	require.NoError(t, json.Unmarshal(raw, &md))
	assert.Equal(t, "u-1", md.UserID)
	assert.Equal(t, "command Test", md.CausationID)
}

// tailer reads the positions of one run's scratch streams in order, the way the projection
// runner reads the store: always after the last position it has seen.
type tailer struct {
	pg      *postgres.Postgres
	pattern string
	cursor  int64
	seen    map[int64]bool
}

func (tl *tailer) read(ctx context.Context, t *testing.T) {
	t.Helper()

	rows, err := tl.pg.Executor(ctx).Query(ctx,
		"SELECT position FROM events WHERE stream_id LIKE $1 AND position > $2 ORDER BY position", tl.pattern, tl.cursor)
	require.NoError(t, err)

	defer rows.Close()

	for rows.Next() {
		var p int64
		require.NoError(t, rows.Scan(&p))

		tl.seen[p] = true
		tl.cursor = p
	}

	require.NoError(t, rows.Err())
}

// A reader that tails the store by position must never pass a position that commits later. A
// slow writer appends inside an open transaction and waits; a second writer appends meanwhile; the
// reader reads; then the slow writer commits. Without the append lock the second writer commits a
// higher position first, the reader moves past the slow one's lower position, and never reads it.
func TestAWriteCommittedLateIsStillReadInOrder(t *testing.T) {
	ctx := context.Background()
	pg := newPostgres(t)
	store := newSuiteStore(pg)
	run := uuid.NewString()
	tl := &tailer{pg: pg, pattern: `\_note-` + run + `-%`, seen: map[int64]bool{}}

	var slowPos, fastPos int64

	saved, release := make(chan struct{}), make(chan struct{})
	slowDone, fastDone := make(chan error, 1), make(chan error, 1)

	go func() {
		slowDone <- pg.InTx(ctx, func(ctx context.Context) error {
			var err error
			if slowPos, err = store.Save(ctx, eventstoretest.NewNote(run+"-slow", "slow")); err != nil {
				return err
			}

			close(saved)
			<-release

			return nil
		})
	}()

	<-saved

	go func() {
		position, err := store.Save(ctx, eventstoretest.NewNote(run+"-fast", "fast"))
		fastPos = position

		fastDone <- err
	}()

	select {
	case err := <-fastDone:
		fastDone <- err
	case <-time.After(lateWriteWait):
	}

	tl.read(ctx, t)
	close(release)
	require.NoError(t, <-slowDone)
	require.NoError(t, <-fastDone)
	tl.read(ctx, t)

	assert.True(t, tl.seen[slowPos], "the slow write at %d was passed by the reader", slowPos)
	assert.True(t, tl.seen[fastPos])
	assert.Less(t, slowPos, fastPos)
}
