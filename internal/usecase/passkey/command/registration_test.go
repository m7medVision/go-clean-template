package command_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/ceremony/ceremonytest"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinishPasskeyRegistrationRecordsTheCredential(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	token, body := f.attestation(t)

	written, err := command.FinishPasskeyRegistrationHandler{Ceremony: f.ceremony, Store: f.store, Now: now}.Handle(
		context.Background(), command.FinishPasskeyRegistration{UserID: f.user.ID, Token: token, Name: " Laptop ", Body: body})
	require.NoError(t, err)

	id := entity.PasskeyID(f.cred.ID)
	assert.Equal(t, id.String(), written.ID)

	events := f.events(id)
	require.Len(t, events, 1)

	registered, ok := events[0].(entity.PasskeyRegistered)
	require.True(t, ok)
	assert.Equal(t, f.user.ID, registered.UserID)
	assert.Equal(t, "Laptop", registered.Name)
	assert.Equal(t, nowStored(), registered.At)

	var record webauthn.Credential

	require.NoError(t, json.Unmarshal(registered.Credential, &record))
	assert.Equal(t, f.cred.ID, record.ID)
	assert.NotEmpty(t, record.PublicKey)
}

func TestARegistrationWithoutANameGetsTheDefault(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	token, body := f.attestation(t)

	_, err := command.FinishPasskeyRegistrationHandler{Ceremony: f.ceremony, Store: f.store}.Handle(context.Background(),
		command.FinishPasskeyRegistration{UserID: f.user.ID, Token: token, Name: "  ", Body: body})
	require.NoError(t, err)

	registered, ok := f.events(entity.PasskeyID(f.cred.ID))[0].(entity.PasskeyRegistered)
	require.True(t, ok)
	assert.Equal(t, "Passkey", registered.Name)
}

func TestRegisteringACredentialTwiceIsErrPasskeyExists(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.register(t)

	token, body := f.attestation(t)

	_, err := command.FinishPasskeyRegistrationHandler{Ceremony: f.ceremony, Store: f.store}.Handle(context.Background(),
		command.FinishPasskeyRegistration{UserID: f.user.ID, Token: token, Name: "Again", Body: body})

	require.ErrorIs(t, err, entity.ErrPasskeyExists)
	assert.Len(t, f.events(entity.PasskeyID(f.cred.ID)), 1)
}

func TestAnInvalidRegistrationRecordsNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	token, body := f.attestation(t)

	_, err := command.FinishPasskeyRegistrationHandler{Ceremony: f.ceremony, Store: f.store}.Handle(context.Background(),
		command.FinishPasskeyRegistration{UserID: f.user.ID, Token: ceremonytest.Tamper(t, token), Name: "x", Body: body})

	require.ErrorIs(t, err, entity.ErrPasskeyCeremonyInvalid)
	assert.Empty(t, f.events(entity.PasskeyID(f.cred.ID)))
}

func TestRenamePasskeyRecordsTheNewName(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.register(t)

	written, err := command.RenamePasskeyHandler{Store: f.store, Now: now}.Handle(context.Background(),
		command.RenamePasskey{UserID: f.user.ID, ID: id.String(), Name: "  "})
	require.NoError(t, err)
	assert.Equal(t, id.String(), written.ID)

	assert.Equal(t, entity.PasskeyRenamed{PasskeyID: id, Name: "Passkey", At: nowStored()}, f.events(id)[1])
}

func TestDeletePasskeyRecordsTheDeletion(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.register(t)

	_, err := command.DeletePasskeyHandler{Store: f.store, Now: now}.Handle(context.Background(),
		command.DeletePasskey{UserID: f.user.ID, ID: id.String()})
	require.NoError(t, err)

	assert.Equal(t, entity.PasskeyDeleted{PasskeyID: id, At: nowStored()}, f.events(id)[1])
}

func TestRenameAndDeleteFindOnlyTheUsersOwnPasskey(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.register(t)
	ctx := context.Background()

	for name, target := range map[string]command.DeletePasskey{
		"another user's":    {UserID: newUser().ID, ID: id.String()},
		"a missing one":     {UserID: f.user.ID, ID: entity.PasskeyID{9}.String()},
		"one not base64url": {UserID: f.user.ID, ID: "not base64!"},
	} {
		_, err := command.DeletePasskeyHandler{Store: f.store}.Handle(ctx, target)
		require.ErrorIs(t, err, entity.ErrPasskeyNotFound, name)

		_, err = command.RenamePasskeyHandler{Store: f.store}.Handle(ctx,
			command.RenamePasskey{UserID: target.UserID, ID: target.ID, Name: "x"})
		require.ErrorIs(t, err, entity.ErrPasskeyNotFound, name)
	}

	assert.Len(t, f.events(id), 1)
}
