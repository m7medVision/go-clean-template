package entity

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type counted struct{ By int }

func (counted) EventType() string  { return "Counted" }
func (counted) SchemaVersion() int { return 1 }

// counter is the smallest aggregate: it adds up what it is told to count.
type counter struct {
	AggregateBase

	id    string
	total int
}

func (c *counter) StreamID() string { return "counter-" + c.id }

func (c *counter) Apply(e Event) {
	if ev, ok := e.(counted); ok {
		c.total += ev.By
	}
}

func (c *counter) count(by int) { Record(c, counted{By: by}) }

type counterSnapshot struct {
	Total int `json:"total"`
}

func (*counter) SnapshotVersion() int { return 1 }

func (c *counter) Snapshot() any { return counterSnapshot{Total: c.total} }

func (c *counter) RestoreSnapshot(data json.RawMessage) error {
	var s counterSnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	c.total = s.Total

	return nil
}

func TestRecordAppliesTheEventAndQueuesIt(t *testing.T) {
	t.Parallel()

	c := &counter{id: "c1"}

	c.count(2)
	c.count(3)

	assert.Equal(t, 5, c.total)
	assert.Equal(t, []Event{counted{By: 2}, counted{By: 3}}, c.Pending())
	assert.Equal(t, 0, c.Version(), "the version moves only when the store saves")
}

func TestReplayAppliesStoredEventsWithoutQueueingThem(t *testing.T) {
	t.Parallel()

	c := &counter{id: "c1"}

	Replay(c, 2, counted{By: 4}, counted{By: 1})

	assert.Equal(t, 5, c.total)
	assert.Empty(t, c.Pending())
	assert.Equal(t, 2, c.Version())
}

func TestSavedAdvancesTheVersionAndClearsThePendingEvents(t *testing.T) {
	t.Parallel()

	c := &counter{id: "c1"}
	Replay(c, 3, counted{By: 1})
	c.count(1)
	c.count(1)

	Saved(c)

	assert.Equal(t, 5, c.Version())
	assert.Empty(t, c.Pending())
}

func TestASnapshotterRestoresItsState(t *testing.T) {
	t.Parallel()

	c := &counter{id: "c1"}
	c.count(7)

	var s Snapshotter = c

	data, err := json.Marshal(s.Snapshot())
	require.NoError(t, err)

	restored := &counter{id: "c1"}
	require.NoError(t, restored.RestoreSnapshot(data))
	assert.Equal(t, 7, restored.total)
}

func TestMetadataTravelsInTheContext(t *testing.T) {
	t.Parallel()

	md := Metadata{UserID: "u1", CorrelationID: "trace-1", CausationID: "command CreateTask"}

	assert.Equal(t, md, MetadataFromContext(ContextWithMetadata(context.Background(), md)))
	assert.Equal(t, Metadata{}, MetadataFromContext(context.Background()))
}

func TestEventTypesHaveUniqueNamesAndAVersion(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}

	for _, e := range EventTypes() {
		assert.False(t, seen[e.EventType()], "%s is listed twice", e.EventType())
		assert.GreaterOrEqual(t, e.SchemaVersion(), 1, "%s has no schema version", e.EventType())

		seen[e.EventType()] = true
	}
}
