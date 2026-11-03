// Package eventstoretest holds the tests every repo.EventStore must pass, so the in-memory store
// the command-handler tests use behaves as the Postgres one does.
package eventstoretest

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SnapshotEvery is the snapshot interval the suite's stores are built with.
const SnapshotEvery = 3

const (
	// _writtenAndEdited is the version of a note written and then edited once.
	_writtenAndEdited = 2
	// _edits takes a note to version 7: a snapshot at version 6, then one event after it.
	_edits       = 6
	_afterEdits  = _edits + 1
	_noteV2Shape = 2
)

var errEmailTaken = errors.New("email taken")

// Events are the event types the suite's store must know.
func Events() []entity.Event {
	return []entity.Event{NoteWritten{}, NoteEdited{}}
}

// NewStore builds an empty-enough store for one test: it knows Events() and takes a snapshot every
// SnapshotEvery events. Tests use fresh stream IDs and claim values, so a store may be shared.
type NewStore func(t *testing.T) repo.EventStore

// Run runs the suite against the stores newStore builds.
func Run(t *testing.T, newStore NewStore) {
	t.Helper()

	tests := map[string]func(t *testing.T, s repo.EventStore){
		"save then load replays the stream":        saveThenLoad,
		"save marks the aggregate saved":           saveMarksSaved,
		"saving nothing writes nothing":            saveNothing,
		"a missing stream is ErrStreamNotFound":    missingStream,
		"two writers at one version conflict":      twoWritersConflict,
		"a stream cannot be created twice":         createTwice,
		"positions increase across streams":        positionsIncrease,
		"a claim held by another fails the save":   claimTaken,
		"the same owner can claim again":           claimAgain,
		"a released claim can be taken":            claimReleased,
		"a snapshot load equals a full replay":     snapshotEqualsReplay,
		"a snapshot of another version is ignored": snapshotOtherVersion,
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			test(t, newStore(t))
		})
	}
}

func saveThenLoad(t *testing.T, s repo.EventStore) {
	t.Helper()

	ctx := context.Background()
	n := NewNote(uuid.NewString(), "first")
	n.Edit("second")

	position, err := s.Save(ctx, n)
	require.NoError(t, err)
	assert.Positive(t, position)

	loaded := &Note{}
	require.NoError(t, s.Load(ctx, n.StreamID(), loaded))
	assert.Equal(t, n.ID, loaded.ID)
	assert.Equal(t, "second", loaded.Text)
	assert.Equal(t, _writtenAndEdited, loaded.Version())
	assert.Empty(t, loaded.Pending())
}

func saveMarksSaved(t *testing.T, s repo.EventStore) {
	t.Helper()

	n := NewNote(uuid.NewString(), "first")
	n.Edit("second")

	_, err := s.Save(context.Background(), n)
	require.NoError(t, err)

	assert.Equal(t, _writtenAndEdited, n.Version())
	assert.Empty(t, n.Pending())
}

func saveNothing(t *testing.T, s repo.EventStore) {
	t.Helper()

	ctx := context.Background()
	n := NewNote(uuid.NewString(), "first")
	_, err := s.Save(ctx, n)
	require.NoError(t, err)

	position, err := s.Save(ctx, n)
	require.NoError(t, err)
	assert.Zero(t, position)
}

func missingStream(t *testing.T, s repo.EventStore) {
	t.Helper()

	err := s.Load(context.Background(), NoteStreamID(uuid.NewString()), &Note{})

	require.ErrorIs(t, err, entity.ErrStreamNotFound)
}

func twoWritersConflict(t *testing.T, s repo.EventStore) {
	t.Helper()

	ctx := context.Background()
	n := NewNote(uuid.NewString(), "first")
	_, err := s.Save(ctx, n)
	require.NoError(t, err)

	a, b := &Note{}, &Note{}
	require.NoError(t, s.Load(ctx, n.StreamID(), a))
	require.NoError(t, s.Load(ctx, n.StreamID(), b))

	a.Edit("by a")
	b.Edit("by b")

	_, err = s.Save(ctx, a)
	require.NoError(t, err)

	_, err = s.Save(ctx, b)
	require.ErrorIs(t, err, entity.ErrConcurrencyConflict)

	loaded := &Note{}
	require.NoError(t, s.Load(ctx, n.StreamID(), loaded))
	assert.Equal(t, "by a", loaded.Text)
}

func createTwice(t *testing.T, s repo.EventStore) {
	t.Helper()

	ctx := context.Background()
	id := uuid.NewString()

	_, err := s.Save(ctx, NewNote(id, "first"))
	require.NoError(t, err)

	_, err = s.Save(ctx, NewNote(id, "again"))
	require.ErrorIs(t, err, entity.ErrConcurrencyConflict)
}

func positionsIncrease(t *testing.T, s repo.EventStore) {
	t.Helper()

	ctx := context.Background()

	first, err := s.Save(ctx, NewNote(uuid.NewString(), "a"))
	require.NoError(t, err)

	second, err := s.Save(ctx, NewNote(uuid.NewString(), "b"))
	require.NoError(t, err)

	assert.Greater(t, second, first)
}

func emailClaim(email, owner string) entity.Claim {
	return entity.Claim{Scope: "test.email", Value: email, Owner: owner, Taken: errEmailTaken}
}

func claimTaken(t *testing.T, s repo.EventStore) {
	t.Helper()

	ctx := context.Background()
	email := uuid.NewString() + "@example.com"
	a := NewNote(uuid.NewString(), "a")

	_, err := s.Save(ctx, a, emailClaim(email, a.ID))
	require.NoError(t, err)

	b := NewNote(uuid.NewString(), "b")
	_, err = s.Save(ctx, b, emailClaim(email, b.ID))
	require.ErrorIs(t, err, errEmailTaken)

	require.ErrorIs(t, s.Load(ctx, b.StreamID(), &Note{}), entity.ErrStreamNotFound, "nothing of the failed save is kept")

	owner, err := s.ClaimOwner(ctx, "test.email", email)
	require.NoError(t, err)
	assert.Equal(t, a.ID, owner)
}

func claimAgain(t *testing.T, s repo.EventStore) {
	t.Helper()

	ctx := context.Background()
	email := uuid.NewString() + "@example.com"
	n := NewNote(uuid.NewString(), "a")

	_, err := s.Save(ctx, n, emailClaim(email, n.ID))
	require.NoError(t, err)

	n.Edit("b")
	_, err = s.Save(ctx, n, emailClaim(email, n.ID))
	require.NoError(t, err)
}

func claimReleased(t *testing.T, s repo.EventStore) {
	t.Helper()

	ctx := context.Background()
	email := uuid.NewString() + "@example.com"
	a := NewNote(uuid.NewString(), "a")

	_, err := s.Save(ctx, a, emailClaim(email, a.ID))
	require.NoError(t, err)

	a.Edit("leaving")
	release := emailClaim(email, a.ID)
	release.Release = true
	_, err = s.Save(ctx, a, release)
	require.NoError(t, err)

	_, err = s.ClaimOwner(ctx, "test.email", email)
	require.ErrorIs(t, err, entity.ErrClaimNotFound)

	b := NewNote(uuid.NewString(), "b")
	_, err = s.Save(ctx, b, emailClaim(email, b.ID))
	require.NoError(t, err)
}

// writeEdits saves a note and then each edit on its own, so the stream crosses the snapshot
// interval on the way.
func writeEdits(t *testing.T, s repo.EventStore, edits int) *Note {
	t.Helper()

	ctx := context.Background()
	n := NewNote(uuid.NewString(), "edit 0")
	_, err := s.Save(ctx, n)
	require.NoError(t, err)

	for i := 1; i <= edits; i++ {
		n.Edit("edit " + strconv.Itoa(i))
		_, err = s.Save(ctx, n)
		require.NoError(t, err)
	}

	return n
}

func snapshotEqualsReplay(t *testing.T, s repo.EventStore) {
	t.Helper()

	n := writeEdits(t, s, _edits)

	loaded := &Note{}
	require.NoError(t, s.Load(context.Background(), n.StreamID(), loaded))

	assert.Equal(t, n.Text, loaded.Text)
	assert.Equal(t, n.Edits, loaded.Edits)
	assert.Equal(t, _afterEdits, loaded.Version())
	assert.Equal(t, 1, loaded.Applied, "only the event after the snapshot is replayed")
}

func snapshotOtherVersion(t *testing.T, s repo.EventStore) {
	t.Helper()

	n := writeEdits(t, s, _edits)

	loaded := &NoteV2{}
	require.NoError(t, s.Load(context.Background(), n.StreamID(), loaded))

	assert.Equal(t, n.Text, loaded.Text)
	assert.Equal(t, _afterEdits, loaded.Version())
	assert.Equal(t, _afterEdits, loaded.Applied, "a snapshot of another shape is ignored and the stream replayed")
}

// NoteWritten starts a note.
type NoteWritten struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// EventType -.
func (NoteWritten) EventType() string { return "NoteWritten" }

// SchemaVersion -.
func (NoteWritten) SchemaVersion() int { return 1 }

// NoteEdited replaces a note's text.
type NoteEdited struct {
	Text string `json:"text"`
}

// EventType -.
func (NoteEdited) EventType() string { return "NoteEdited" }

// SchemaVersion -.
func (NoteEdited) SchemaVersion() int { return 1 }

// Note is the suite's aggregate. Applied counts the events applied to this value, so a test can
// tell a snapshot load from a full replay.
type Note struct {
	entity.AggregateBase

	ID      string
	Text    string
	Edits   int
	Applied int
}

// NoteStreamID is the stream of the note with id: a scratch stream (leading underscore), which the
// projection runner never delivers, so the suite can run against the database a worker reads.
func NoteStreamID(id string) string { return "_note-" + id }

// NewNote writes a note.
func NewNote(id, text string) *Note {
	n := &Note{}
	entity.Record(n, NoteWritten{ID: id, Text: text})

	return n
}

// Edit replaces the note's text.
func (n *Note) Edit(text string) { entity.Record(n, NoteEdited{Text: text}) }

// StreamID -.
func (n *Note) StreamID() string { return NoteStreamID(n.ID) }

// Apply -.
func (n *Note) Apply(e entity.Event) {
	n.Applied++

	switch ev := e.(type) {
	case NoteWritten:
		n.ID, n.Text = ev.ID, ev.Text
	case NoteEdited:
		n.Text = ev.Text
		n.Edits++
	}
}

type noteSnapshot struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Edits int    `json:"edits"`
}

// SnapshotVersion -.
func (*Note) SnapshotVersion() int { return 1 }

// Snapshot -.
func (n *Note) Snapshot() any { return noteSnapshot{ID: n.ID, Text: n.Text, Edits: n.Edits} }

// RestoreSnapshot -.
func (n *Note) RestoreSnapshot(data json.RawMessage) error {
	var s noteSnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	n.ID, n.Text, n.Edits = s.ID, s.Text, s.Edits

	return nil
}

// NoteV2 is Note after its snapshot changed shape.
type NoteV2 struct {
	Note
}

// SnapshotVersion -.
func (*NoteV2) SnapshotVersion() int { return _noteV2Shape }
