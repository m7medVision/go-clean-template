package entity_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
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
	assert.ElementsMatch(t, []entity.Claim{
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
	assert.Equal(t, []entity.Claim{
		{
			Scope: entity.ClaimOIDCIdentity, Value: entity.OIDCIdentity(_issuer, _subject), Owner: _userID,
			Taken: entity.ErrOIDCIdentityTaken,
		},
		{Scope: entity.ClaimUserEmail, Value: "jo@example.com", Owner: _userID, Taken: entity.ErrAccountConflict},
		{Scope: entity.ClaimUserUsername, Value: "jo", Owner: _userID, Taken: entity.ErrAccountConflict},
	}, claims, "the identity first, so a racing first sign-in reads as the identity taken")
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
	assert.ElementsMatch(t, []entity.Claim{
		{Scope: entity.ClaimUserEmail, Value: "jo@example.com", Owner: _userID, Taken: entity.ErrUserAlreadyExists},
		{Scope: entity.ClaimUserUsername, Value: "jo", Owner: _userID, Taken: entity.ErrUserAlreadyExists},
		{
			Scope: entity.ClaimOIDCIdentity, Value: entity.OIDCIdentity(_issuer, _subject), Owner: _userID,
			Taken: entity.ErrOIDCIdentityTaken,
		},
	}, claims)
}
