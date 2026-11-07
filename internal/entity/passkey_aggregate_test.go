package entity_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func passkeyID() entity.PasskeyID { return entity.PasskeyID{1, 2, 3, 4} }

func record() json.RawMessage { return json.RawMessage(`{"id":"AQIDBA"}`) }

func registered() *entity.PasskeyAggregate {
	a := &entity.PasskeyAggregate{}
	entity.Replay(a, 1, entity.PasskeyRegistered{
		PasskeyID: passkeyID(), UserID: _owner, Name: "Laptop", Credential: record(), At: createdAt(),
	})

	return a
}

func TestRegisterRecordsPasskeyRegistered(t *testing.T) {
	t.Parallel()

	a := &entity.PasskeyAggregate{}

	require.NoError(t, a.Register(passkeyID(), _owner, "Laptop", record(), createdAt()))

	assert.Equal(t, []entity.Event{entity.PasskeyRegistered{
		PasskeyID: passkeyID(), UserID: _owner, Name: "Laptop", Credential: record(), At: createdAt(),
	}}, a.Pending())
	assert.Equal(t, "passkey-AQIDBA", a.StreamID())
	assert.True(t, a.OwnedBy(_owner))
	assert.False(t, a.OwnedBy(_other))
}

func TestRegisteringALiveCredentialAgainIsErrPasskeyExists(t *testing.T) {
	t.Parallel()

	a := registered()

	require.ErrorIs(t, a.Register(passkeyID(), _other, "Other", record(), laterAt()), entity.ErrPasskeyExists)
	assert.Empty(t, a.Pending())
}

func TestADeletedCredentialCanBeRegisteredAgain(t *testing.T) {
	t.Parallel()

	a := registered()
	require.NoError(t, a.Delete(_owner, laterAt()))
	require.False(t, a.Live())

	require.NoError(t, a.Register(passkeyID(), _owner, "Again", record(), laterAt()))
	assert.True(t, a.Live())
}

func TestRenameAndDeleteAreTheOwnersOnly(t *testing.T) {
	t.Parallel()

	a := registered()

	require.ErrorIs(t, a.Rename(_other, "x", laterAt()), entity.ErrPasskeyNotFound)
	require.ErrorIs(t, a.Delete(_other, laterAt()), entity.ErrPasskeyNotFound)
	assert.Empty(t, a.Pending())

	require.NoError(t, a.Rename(_owner, "Phone", laterAt()))
	require.NoError(t, a.Delete(_owner, laterAt()))

	assert.Equal(t, []entity.Event{
		entity.PasskeyRenamed{PasskeyID: passkeyID(), Name: "Phone", At: laterAt()},
		entity.PasskeyDeleted{PasskeyID: passkeyID(), At: laterAt()},
	}, a.Pending())
}

func TestADeletedOrMissingPasskeyIsNotFound(t *testing.T) {
	t.Parallel()

	for _, a := range []*entity.PasskeyAggregate{{}, func() *entity.PasskeyAggregate {
		d := registered()
		entity.Replay(d, 2, entity.PasskeyDeleted{PasskeyID: passkeyID(), At: laterAt()})

		return d
	}()} {
		require.ErrorIs(t, a.Rename(_owner, "x", laterAt()), entity.ErrPasskeyNotFound)
		require.ErrorIs(t, a.Delete(_owner, laterAt()), entity.ErrPasskeyNotFound)
		require.ErrorIs(t, a.Use(_owner, record(), laterAt()), entity.ErrPasskeyNotFound)
	}
}

func TestUseRecordsTheCredentialTheNextSignInVerifiesAgainst(t *testing.T) {
	t.Parallel()

	a := registered()
	updated := json.RawMessage(`{"id":"AQIDBA","signCount":2}`)

	require.ErrorIs(t, a.Use(_other, updated, laterAt()), entity.ErrPasskeyNotFound, "another user's passkey")
	require.NoError(t, a.Use(_owner, updated, laterAt()))

	assert.Equal(t, []entity.Event{entity.PasskeyUsed{PasskeyID: passkeyID(), Credential: updated, At: laterAt()}}, a.Pending())
	assert.JSONEq(t, string(updated), string(a.Credential()))
}

func TestImportPasskeyAggregateRecordsTheRow(t *testing.T) {
	t.Parallel()

	used := laterAt()
	row := entity.Passkey{
		ID: passkeyID(), UserID: _owner, Name: "Laptop", Credential: record(), CreatedAt: createdAt(), LastUsedAt: &used,
	}

	a := entity.ImportPasskeyAggregate(&row)

	assert.Equal(t, []entity.Event{entity.PasskeyImported{
		PasskeyID: passkeyID(), UserID: _owner, Name: "Laptop", Credential: record(), CreatedAt: createdAt(), LastUsedAt: &used,
	}}, a.Pending())
}

func TestPasskeyAggregateSnapshotKeepsTheOwnerAndTheCredential(t *testing.T) {
	t.Parallel()

	a := registered()

	data, err := json.Marshal(a.Snapshot())
	require.NoError(t, err)

	restored := &entity.PasskeyAggregate{}
	require.NoError(t, restored.RestoreSnapshot(data))
	assert.Equal(t, a.StreamID(), restored.StreamID())
	assert.True(t, restored.OwnedBy(_owner))
	assert.JSONEq(t, string(record()), string(restored.Credential()))
}

// A snapshot stored before the aggregate kept its own state held the whole passkey; it still loads.
func TestAPasskeySnapshotOfTheFirstShapeStillLoads(t *testing.T) {
	t.Parallel()

	restored := &entity.PasskeyAggregate{}
	require.NoError(t, restored.RestoreSnapshot([]byte(`{"passkey":{"id":"AQIDBA","name":"Laptop",
		"created_at":"2026-01-01T09:00:00Z","last_used_at":null},"user_id":"user-1","credential":{"id":"AQIDBA"},
		"registered":true,"deleted":false}`)))

	assert.Equal(t, "passkey-AQIDBA", restored.StreamID())
	assert.True(t, restored.OwnedBy(_owner))
	assert.JSONEq(t, string(record()), string(restored.Credential()))
}

func TestAPasskeysNameIsTrimmedDefaultedAndBounded(t *testing.T) {
	t.Parallel()

	a := &entity.PasskeyAggregate{}
	require.NoError(t, a.Register(passkeyID(), _owner, "  Laptop ", record(), createdAt()))
	require.NoError(t, a.Rename(_owner, "   ", laterAt()))
	require.ErrorIs(t, a.Rename(_owner, strings.Repeat("a", entity.MaxPasskeyNameLen+1), laterAt()),
		entity.ErrInvalidPasskeyName)

	assert.Equal(t, []entity.Event{
		entity.PasskeyRegistered{PasskeyID: passkeyID(), UserID: _owner, Name: "Laptop", Credential: record(), At: createdAt()},
		entity.PasskeyRenamed{PasskeyID: passkeyID(), Name: "Passkey", At: laterAt()},
	}, a.Pending())
}

func TestThePasskeyEventsAreRegistered(t *testing.T) {
	t.Parallel()

	names := map[string]bool{}
	for _, e := range entity.EventTypes() {
		names[e.EventType()] = true
	}

	for _, e := range []entity.Event{
		entity.PasskeyRegistered{}, entity.PasskeyRenamed{}, entity.PasskeyUsed{}, entity.PasskeyDeleted{}, entity.PasskeyImported{},
	} {
		assert.True(t, names[e.EventType()], e.EventType())
	}
}
