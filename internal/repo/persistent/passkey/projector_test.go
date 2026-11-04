package passkey_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/passkey"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errWrite = errors.New("write failed")

// fakeQuerier records the passkey writes; any other query panics on the nil embedded interface.
type fakeQuerier struct {
	queries.Querier

	inserted []queries.InsertPasskeyRowParams
	names    []queries.SetPasskeyNameParams
	usages   []queries.SetPasskeyUsageParams
	deleted  []entity.PasskeyID
	err      error
}

//nolint:gocritic // the signature is sqlc's Querier, which takes its params by value
func (f *fakeQuerier) InsertPasskeyRow(_ context.Context, arg queries.InsertPasskeyRowParams) error {
	f.inserted = append(f.inserted, arg)

	return f.err
}

func (f *fakeQuerier) SetPasskeyName(_ context.Context, arg queries.SetPasskeyNameParams) error {
	f.names = append(f.names, arg)

	return f.err
}

func (f *fakeQuerier) SetPasskeyUsage(_ context.Context, arg queries.SetPasskeyUsageParams) error {
	f.usages = append(f.usages, arg)

	return f.err
}

func (f *fakeQuerier) DeletePasskeyRow(_ context.Context, id entity.PasskeyID) error {
	f.deleted = append(f.deleted, id)

	return f.err
}

func at() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC) }

func id() entity.PasskeyID { return entity.PasskeyID{1, 2, 3, 4} }

func credential() json.RawMessage { return json.RawMessage(`{"id":"AQIDBA"}`) }

func project(t *testing.T, q *fakeQuerier, e entity.Event) error {
	t.Helper()

	return passkey.Project(context.Background(), q, &entity.RecordedEvent{Position: 1, StreamID: "passkey-AQIDBA", Event: e})
}

func TestPasskeyRegisteredInsertsARow(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}

	require.NoError(t, project(t, q, entity.PasskeyRegistered{PasskeyID: id(), UserID: "u1", Name: "Laptop", Credential: credential(), At: at()}))

	assert.Equal(t, []queries.InsertPasskeyRowParams{{
		ID: id(), UserID: "u1", Name: "Laptop", Credential: credential(), CreatedAt: at(),
	}}, q.inserted)
}

func TestPasskeyImportedInsertsTheRowItCarries(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}
	used := at().Add(time.Hour)

	require.NoError(t, project(t, q, entity.PasskeyImported{
		PasskeyID: id(), UserID: "u1", Name: "Laptop", Credential: credential(), CreatedAt: at(), LastUsedAt: &used,
	}))

	assert.Equal(t, []queries.InsertPasskeyRowParams{{
		ID: id(), UserID: "u1", Name: "Laptop", Credential: credential(), CreatedAt: at(), LastUsedAt: &used,
	}}, q.inserted)
}

func TestPasskeyRenamedUsedAndDeletedChangeTheRow(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}

	require.NoError(t, project(t, q, entity.PasskeyRenamed{PasskeyID: id(), Name: "Phone", At: at()}))
	require.NoError(t, project(t, q, entity.PasskeyUsed{PasskeyID: id(), Credential: credential(), At: at()}))
	require.NoError(t, project(t, q, entity.PasskeyDeleted{PasskeyID: id(), At: at()}))

	used := at()

	assert.Equal(t, []queries.SetPasskeyNameParams{{ID: id(), Name: "Phone"}}, q.names)
	assert.Equal(t, []queries.SetPasskeyUsageParams{{ID: id(), Credential: credential(), LastUsedAt: &used}}, q.usages)
	assert.Equal(t, []entity.PasskeyID{id()}, q.deleted)
}

func TestThePasskeyProjectorReturnsAWriteFailure(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, project(t, &fakeQuerier{err: errWrite}, entity.PasskeyDeleted{PasskeyID: id()}), errWrite)
}

func TestThePasskeyProjectorIgnoresOtherEvents(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}

	require.NoError(t, project(t, q, entity.TaskDeleted{TaskID: "t1"}))
	assert.Empty(t, q.inserted)
}

func TestThePasskeyProjectorOwnsTheCredentialsTable(t *testing.T) {
	t.Parallel()

	p := passkey.NewProjector(nil)

	assert.Equal(t, "passkey", p.Name())
	assert.Equal(t, []string{"webauthn_credentials"}, p.Tables())
}
