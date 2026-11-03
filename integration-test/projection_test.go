package integration_test

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/eventstoretest"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/projection"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/pkg/logger"
	"github.com/m7medvision/go-clean-template/pkg/postgres"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	projectionWait = 5 * time.Second
	projectionTick = 20 * time.Millisecond
)

// runCounter counts how often it was handed each position of one run's scratch notes. Its name
// starts with an underscore, so it shows in no lag metric and no wait.
type runCounter struct {
	name   string
	prefix string

	mu     sync.Mutex
	counts map[int64]int
}

func newRunCounter(run string) *runCounter {
	return &runCounter{
		name:   "_test-" + run,
		prefix: eventstoretest.NoteStreamID(run),
		counts: map[int64]int{},
	}
}

func (c *runCounter) Name() string { return c.name }

func (*runCounter) Tables() []string { return nil }

func (c *runCounter) Handle(_ context.Context, e *entity.RecordedEvent) error {
	if !strings.HasPrefix(e.StreamID, c.prefix) {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.counts[e.Position]++

	return nil
}

// handled returns the positions handled, and how many times each was.
func (c *runCounter) handled() (positions []int64, counts map[int64]int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	positions = make([]int64, 0, len(c.counts))
	for p := range c.counts {
		positions = append(positions, p)
	}

	slices.Sort(positions)

	counts = make(map[int64]int, len(c.counts))
	maps.Copy(counts, c.counts)

	return positions, counts
}

// handledExactly reports whether the positions handled are *want, no more and no fewer.
func (c *runCounter) handledExactly(want *[]int64) func() bool {
	return func() bool {
		got, _ := c.handled()

		return slices.Equal(got, *want)
	}
}

func startRunner(t *testing.T, pg *postgres.Postgres, c *runCounter) func() {
	t.Helper()

	events := append(entity.EventTypes(), eventstoretest.Events()...)
	l := logger.New("error")
	store := projection.NewStore(pg, eventstore.NewReader(pg, events).IncludingScratch(), l)

	// An hour between polls: only the LISTEN notification can wake the runner in time.
	runner := projection.New(l, store, projection.PollInterval(time.Hour))
	runner.Add(c)
	runner.Start()

	return func() {
		runner.Shutdown()
		store.Close()
	}
}

func saveNotes(ctx context.Context, t *testing.T, pg *postgres.Postgres, run string, from, to int) []int64 {
	t.Helper()

	store := newSuiteStore(pg)
	positions := make([]int64, 0, to-from)

	for i := from; i < to; i++ {
		p, err := store.Save(ctx, eventstoretest.NewNote(run+"-"+uuid.NewString()[:8], "x"))
		require.NoError(t, err)

		positions = append(positions, p)
	}

	return positions
}

func TestTheRunnerDeliversEachEventOnceResumesAndRebuilds(t *testing.T) {
	ctx := context.Background()
	pg := newPostgres(t)
	run := uuid.NewString()
	c := newRunCounter(run)

	stop := startRunner(t, pg, c)
	want := saveNotes(ctx, t, pg, run, 0, 3)

	require.Eventually(t, c.handledExactly(&want), projectionWait, projectionTick,
		"a notification wakes the runner for each append")

	// A restarted runner resumes after its checkpoint.
	stop()

	want = append(want, saveNotes(ctx, t, pg, run, 3, 5)...)
	stop = startRunner(t, pg, c)

	require.Eventually(t, c.handledExactly(&want), projectionWait, projectionTick)

	_, counts := c.handled()
	for _, p := range want {
		assert.Equal(t, 1, counts[p], "position %d delivered more than once", p)
	}

	// A rebuild resets the checkpoint, and the running runner replays from the first event.
	require.NoError(t, projection.Rebuild(ctx, pg, c.name, c))

	require.Eventually(t, func() bool {
		_, counts := c.handled()
		for _, p := range want {
			if counts[p] != 2 {
				return false
			}
		}

		return true
	}, projectionWait, projectionTick, "every event replayed once after the rebuild")

	stop()
}

func TestTheLagCollectorReportsEachCheckpoint(t *testing.T) {
	ctx := context.Background()
	pg := newPostgres(t)
	name := "lagtest-" + uuid.NewString()
	q := queries.New(pg.Pool)

	require.NoError(t, q.EnsureCheckpoint(ctx, name))
	t.Cleanup(func() {
		_, err := pg.Pool.Exec(context.Background(), "DELETE FROM projection_checkpoints WHERE name = $1", name)
		assert.NoError(t, err)
	})

	reg := prometheus.NewRegistry()
	require.NoError(t, reg.Register(projection.NewLagCollector(pg)))

	families, err := reg.Gather()
	require.NoError(t, err)

	var found bool

	for _, f := range families {
		for _, m := range f.GetMetric() {
			for _, label := range m.GetLabel() {
				found = found || (f.GetName() == "projection_lag_events" && label.GetValue() == name)
			}
		}
	}

	assert.True(t, found, "projection_lag_events has a series for %s", name)
}

func TestCheckpointsReachedNeedsEveryNamedProjector(t *testing.T) {
	ctx := context.Background()
	pg := newPostgres(t)
	a, b := "_reached-"+uuid.NewString(), "_reached-"+uuid.NewString()
	q := queries.New(pg.Pool)
	checkpoints := projection.NewCheckpoints(pg)

	t.Cleanup(func() {
		_, err := pg.Pool.Exec(context.Background(), "DELETE FROM projection_checkpoints WHERE name = ANY($1)", []string{a, b})
		assert.NoError(t, err)
	})

	reached, err := checkpoints.Reached(ctx, []string{a, b}, 5)
	require.NoError(t, err)
	assert.False(t, reached, "projectors with no checkpoint have reached nothing")

	require.NoError(t, q.EnsureCheckpoint(ctx, a))
	require.NoError(t, q.EnsureCheckpoint(ctx, b))
	_, err = q.AdvanceCheckpoint(ctx, queries.AdvanceCheckpointParams{Name: a, FromPosition: 0, ToPosition: 5})
	require.NoError(t, err)

	reached, err = checkpoints.Reached(ctx, []string{a, b}, 5)
	require.NoError(t, err)
	assert.False(t, reached, "b is still at 0")

	_, err = q.AdvanceCheckpoint(ctx, queries.AdvanceCheckpointParams{Name: b, FromPosition: 0, ToPosition: 6})
	require.NoError(t, err)

	reached, err = checkpoints.Reached(ctx, []string{a, b}, 5)
	require.NoError(t, err)
	assert.True(t, reached)
}
