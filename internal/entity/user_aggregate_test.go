package entity_test

import (
	"encoding/json"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	_userID  = "user-1"
	_issuer  = "https://sso.example.com"
	_subject = "sub-1"
)

func TestNewUserAggregateRecordsUserRegisteredAndClaimsItsNames(t *testing.T) {
	t.Parallel()

	user, claims := entity.NewUserAggregate(_userID, "jo", "jo@example.com", "hash", createdAt())

	assert.Equal(t, []entity.Event{entity.UserRegistered{
		UserID: _userID, Username: "jo", Email: "jo@example.com", PasswordHash: "hash", At: createdAt(),
	}}, user.Pending())
	assert.Equal(t, "user-"+_userID, user.StreamID())
	assert.Equal(t, entity.User{
		ID: _userID, Username: "jo", Email: "jo@example.com", PasswordHash: "hash", CreatedAt: createdAt(), UpdatedAt: createdAt(),
	}, user.State())
	assert.Equal(t, []entity.Claim{
		{Scope: entity.ClaimUserEmail, Value: "jo@example.com", Owner: _userID, Taken: entity.ErrUserAlreadyExists},
		{Scope: entity.ClaimUserUsername, Value: "jo", Owner: _userID, Taken: entity.ErrUserAlreadyExists},
	}, claims)
}

func TestNewOIDCUserAggregateHasNoPasswordAndClaimsItsIdentity(t *testing.T) {
	t.Parallel()

	user, claims := entity.NewOIDCUserAggregate(_userID, "jo", "jo@example.com", _issuer, _subject, createdAt())

	assert.Equal(t, []entity.Event{entity.UserRegisteredViaOIDC{
		UserID: _userID, Username: "jo", Email: "jo@example.com", Issuer: _issuer, Subject: _subject, At: createdAt(),
	}}, user.Pending())
	assert.Empty(t, user.State().PasswordHash)
	assert.Equal(t, []entity.Claim{
		{Scope: entity.ClaimUserEmail, Value: "jo@example.com", Owner: _userID, Taken: entity.ErrAccountConflict},
		{Scope: entity.ClaimUserUsername, Value: "jo", Owner: _userID, Taken: entity.ErrAccountConflict},
		{
			Scope: entity.ClaimOIDCIdentity, Value: entity.OIDCIdentity(_issuer, _subject), Owner: _userID,
			Taken: entity.ErrUserAlreadyExists,
		},
	}, claims)
}

func TestImportUserAggregateRecordsTheRowAndItsIdentities(t *testing.T) {
	t.Parallel()

	row := entity.User{
		ID: _userID, Username: "jo", Email: "jo@example.com", PasswordHash: "hash", CreatedAt: createdAt(), UpdatedAt: laterAt(),
	}
	identities := []entity.UserIdentity{{Issuer: _issuer, Subject: _subject, CreatedAt: laterAt()}}

	user, claims := entity.ImportUserAggregate(&row, identities)

	assert.Equal(t, []entity.Event{entity.UserImported{
		UserID: _userID, Username: "jo", Email: "jo@example.com", PasswordHash: "hash",
		CreatedAt: createdAt(), UpdatedAt: laterAt(), Identities: identities,
	}}, user.Pending())
	assert.Equal(t, row, user.State())
	assert.Len(t, claims, 3)
	assert.Equal(t, entity.OIDCIdentity(_issuer, _subject), claims[2].Value)
}

func TestAUserIsRebuiltFromEachOfItsFirstEvents(t *testing.T) {
	t.Parallel()

	for _, first := range []entity.Event{
		entity.UserRegistered{UserID: _userID, Username: "jo", Email: "jo@example.com", PasswordHash: "h", At: createdAt()},
		entity.UserRegisteredViaOIDC{UserID: _userID, Username: "jo", Email: "jo@example.com", At: createdAt()},
		entity.UserImported{UserID: _userID, Username: "jo", Email: "jo@example.com", CreatedAt: createdAt(), UpdatedAt: createdAt()},
	} {
		user := &entity.UserAggregate{}
		entity.Replay(user, 1, first)

		assert.Equal(t, _userID, user.State().ID, first.EventType())
		assert.Equal(t, "jo@example.com", user.State().Email, first.EventType())
	}
}

func TestUserAggregateSnapshotRestoresTheSameUser(t *testing.T) {
	t.Parallel()

	user, _ := entity.NewUserAggregate(_userID, "jo", "jo@example.com", "hash", createdAt())

	data, err := json.Marshal(user.Snapshot())
	require.NoError(t, err)

	restored := &entity.UserAggregate{}
	require.NoError(t, restored.RestoreSnapshot(data))
	assert.Equal(t, user.State(), restored.State())
}

func TestTheUserEventsAreRegistered(t *testing.T) {
	t.Parallel()

	names := map[string]bool{}
	for _, e := range entity.EventTypes() {
		names[e.EventType()] = true
	}

	for _, e := range []entity.Event{entity.UserRegistered{}, entity.UserRegisteredViaOIDC{}, entity.UserImported{}} {
		assert.True(t, names[e.EventType()], e.EventType())
	}
}
