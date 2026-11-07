package entity

import (
	"encoding/json"
	"fmt"
	"time"
)

// The claim scopes that keep users unique: a value claimed in one is held by one user.
const (
	ClaimUserEmail    = "user.email"
	ClaimUserUsername = "user.username"
	ClaimOIDCIdentity = "oidc.identity"
)

// _userSnapshotVersion is the version of userSnapshot's shape.
const _userSnapshotVersion = 1

// UserStreamID is the stream of the user with id.
func UserStreamID(id string) string {
	return "user-" + id
}

// OIDCIdentity is the claim value of an identity at an OpenID Connect provider.
func OIDCIdentity(issuer, subject string) string {
	return issuer + "|" + subject
}

// UserAggregate is a user on the write side, rebuilt from their events. Reads use User, from the
// table the user projector writes.
type UserAggregate struct {
	AggregateBase

	user       User
	identities []UserIdentity
}

var _ Snapshotter = (*UserAggregate)(nil)

// NewUserAggregate registers a user with a password hash, and returns the claims that keep their
// email and username unique: a taken one is ErrUserAlreadyExists.
func NewUserAggregate(id, username, email, passwordHash string, at time.Time) (*UserAggregate, []Claim) {
	u := &UserAggregate{}
	Record(u, UserRegistered{UserID: id, Username: username, Email: email, PasswordHash: passwordHash, At: at})
	claims := u.nameClaims(ErrUserAlreadyExists)

	return u, claims
}

// NewOIDCUserAggregate creates a user on the first SSO sign-in of an identity. A taken identity is
// ErrUserAlreadyExists: a concurrent first sign-in of the same identity won. A taken email or username
// is ErrAccountConflict, since accounts are never linked by email. The identity is claimed first, so a
// race reads as the former even though the winner holds the same email.
func NewOIDCUserAggregate(id, username, email, issuer, subject string, at time.Time) (*UserAggregate, []Claim) {
	u := &UserAggregate{}
	Record(u, UserRegisteredViaOIDC{UserID: id, Username: username, Email: email, Issuer: issuer, Subject: subject, At: at})

	claims := []Claim{u.identityClaim(issuer, subject)}

	return u, append(claims, u.nameClaims(ErrAccountConflict)...)
}

// ImportUserAggregate records a user written before users stored events, with their identities, as
// UserImported, and returns every claim they hold.
func ImportUserAggregate(row *User, identities []UserIdentity) (*UserAggregate, []Claim) {
	u := &UserAggregate{}
	Record(u, UserImported{
		UserID: row.ID, Username: row.Username, Email: row.Email, PasswordHash: row.PasswordHash,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Identities: identities,
	})

	claims := u.nameClaims(ErrUserAlreadyExists)
	for _, i := range identities {
		claims = append(claims, u.identityClaim(i.Issuer, i.Subject))
	}

	return u, claims
}

func (u *UserAggregate) nameClaims(taken error) []Claim {
	return []Claim{
		{Scope: ClaimUserEmail, Value: u.user.Email, Owner: u.user.ID, Taken: taken},
		{Scope: ClaimUserUsername, Value: u.user.Username, Owner: u.user.ID, Taken: taken},
	}
}

func (u *UserAggregate) identityClaim(issuer, subject string) Claim {
	return Claim{Scope: ClaimOIDCIdentity, Value: OIDCIdentity(issuer, subject), Owner: u.user.ID, Taken: ErrUserAlreadyExists}
}

// StreamID -.
func (u *UserAggregate) StreamID() string {
	return UserStreamID(u.user.ID)
}

// ID is the user's ID.
func (u *UserAggregate) ID() string {
	return u.user.ID
}

// State is the user as their events add up.
func (u *UserAggregate) State() User {
	return u.user
}

// Apply -.
func (u *UserAggregate) Apply(e Event) {
	switch ev := e.(type) {
	case UserRegistered:
		u.user = User{
			ID: ev.UserID, Username: ev.Username, Email: ev.Email, PasswordHash: ev.PasswordHash,
			CreatedAt: ev.At, UpdatedAt: ev.At,
		}
	case UserRegisteredViaOIDC:
		u.user = User{ID: ev.UserID, Username: ev.Username, Email: ev.Email, CreatedAt: ev.At, UpdatedAt: ev.At}
		u.identities = []UserIdentity{{Issuer: ev.Issuer, Subject: ev.Subject, CreatedAt: ev.At}}
	case UserImported:
		u.user = User{
			ID: ev.UserID, Username: ev.Username, Email: ev.Email, PasswordHash: ev.PasswordHash,
			CreatedAt: ev.CreatedAt, UpdatedAt: ev.UpdatedAt,
		}
		u.identities = ev.Identities
	}
}

type userSnapshot struct {
	User         User           `json:"user"`
	PasswordHash string         `json:"password_hash"`
	Identities   []UserIdentity `json:"identities"`
}

// SnapshotVersion -.
func (*UserAggregate) SnapshotVersion() int {
	return _userSnapshotVersion
}

// Snapshot -. User's JSON leaves the password hash out, so the snapshot carries it beside.
func (u *UserAggregate) Snapshot() any {
	return userSnapshot{User: u.user, PasswordHash: u.user.PasswordHash, Identities: u.identities}
}

// RestoreSnapshot -.
func (u *UserAggregate) RestoreSnapshot(data json.RawMessage) error {
	var s userSnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("UserAggregate - RestoreSnapshot: %w", err)
	}

	u.user, u.identities = s.User, s.Identities
	u.user.PasswordHash = s.PasswordHash

	return nil
}
