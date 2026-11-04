package eventstore_test

import (
	"encoding/json"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/eventstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type renamed struct {
	Title string `json:"title"`
}

func (renamed) EventType() string  { return "Renamed" }
func (renamed) SchemaVersion() int { return 3 }

type unlisted struct{}

func (unlisted) EventType() string  { return "Unlisted" }
func (unlisted) SchemaVersion() int { return 1 }

// The stored shapes Renamed had: version 1 called the field name, version 2 headline.
func renamedUpcasts() []eventstore.Upcast {
	return []eventstore.Upcast{
		{Type: "Renamed", From: 1, Up: func(payload json.RawMessage) (json.RawMessage, error) {
			var v1 struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(payload, &v1); err != nil {
				return nil, err
			}

			return json.Marshal(map[string]string{"headline": v1.Name})
		}},
		{Type: "Renamed", From: 2, Up: func(payload json.RawMessage) (json.RawMessage, error) {
			var v2 struct {
				Headline string `json:"headline"`
			}
			if err := json.Unmarshal(payload, &v2); err != nil {
				return nil, err
			}

			return json.Marshal(renamed{Title: v2.Headline})
		}},
	}
}

func TestCodecRoundTripsAnEvent(t *testing.T) {
	t.Parallel()

	codec := eventstore.NewCodec([]entity.Event{renamed{}})

	stored, err := codec.Encode(renamed{Title: "a"})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", stored.Type)
	assert.Equal(t, 3, stored.SchemaVersion)

	e, err := codec.Decode(stored)
	require.NoError(t, err)
	assert.Equal(t, renamed{Title: "a"}, e)
}

func TestCodecUpcastsAnOlderVersionStepByStep(t *testing.T) {
	t.Parallel()

	codec := eventstore.NewCodec([]entity.Event{renamed{}}, renamedUpcasts()...)

	e, err := codec.Decode(eventstore.Stored{
		Type: "Renamed", SchemaVersion: 1, Payload: json.RawMessage(`{"name":"old"}`),
	})

	require.NoError(t, err)
	assert.Equal(t, renamed{Title: "old"}, e)
}

func TestCodecRejectsAVersionWithNoUpcaster(t *testing.T) {
	t.Parallel()

	codec := eventstore.NewCodec([]entity.Event{renamed{}})

	_, err := codec.Decode(eventstore.Stored{Type: "Renamed", SchemaVersion: 2, Payload: json.RawMessage(`{}`)})

	require.ErrorIs(t, err, eventstore.ErrNoUpcaster)
}

func TestCodecRejectsAVersionNewerThanTheCode(t *testing.T) {
	t.Parallel()

	codec := eventstore.NewCodec([]entity.Event{renamed{}})

	_, err := codec.Decode(eventstore.Stored{Type: "Renamed", SchemaVersion: 4, Payload: json.RawMessage(`{}`)})

	require.ErrorIs(t, err, eventstore.ErrNewerVersion)
}

func TestCodecRejectsAnUnknownType(t *testing.T) {
	t.Parallel()

	codec := eventstore.NewCodec([]entity.Event{renamed{}})

	_, err := codec.Decode(eventstore.Stored{Type: "Unlisted", SchemaVersion: 1, Payload: json.RawMessage(`{}`)})
	require.ErrorIs(t, err, eventstore.ErrUnknownEvent)

	_, err = codec.Encode(unlisted{})
	require.ErrorIs(t, err, eventstore.ErrUnknownEvent)
}

func TestCodecKnowsEveryDomainEvent(t *testing.T) {
	t.Parallel()

	codec := eventstore.NewCodec(entity.EventTypes())

	for _, e := range entity.EventTypes() {
		stored, err := codec.Encode(e)
		require.NoError(t, err, e.EventType())

		decoded, err := codec.Decode(stored)
		require.NoError(t, err, e.EventType())

		// Compared as stored: a decoded zero decimal is equal to, but not built like, the zero value.
		again, err := codec.Encode(decoded)
		require.NoError(t, err, e.EventType())
		assert.Equal(t, stored, again)
	}
}

func TestASnapshotIsDueWhenTheStreamCrossesTheInterval(t *testing.T) {
	t.Parallel()

	assert.False(t, eventstore.SnapshotDue(0, 2, 3))
	assert.True(t, eventstore.SnapshotDue(2, 3, 3))
	assert.True(t, eventstore.SnapshotDue(2, 7, 3))
	assert.False(t, eventstore.SnapshotDue(3, 5, 3))
	assert.False(t, eventstore.SnapshotDue(0, 100, 0))
}
