package entity

import "time"

// The claim scopes that keep users unique: a value claimed in one is held by one user.
const (
	ClaimUserEmail    = "user.email"
	ClaimUserUsername = "user.username"
	ClaimOIDCIdentity = "oidc.identity"
)

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

	id, username, email string
}

// NewUserAggregate registers a user with a password hash, and returns the claims that keep their
// email and username unique: a taken one is ErrUserAlreadyExists.
func NewUserAggregate(id, username, email, passwordHash string, at time.Time) (*UserAggregate, []Claim) {
	u := &UserAggregate{}
	Record(u, UserRegistered{UserID: id, Username: username, Email: email, PasswordHash: passwordHash, At: at})
	claims := u.nameClaims(ErrUserAlreadyExists)

	return u, claims
}

// NewOIDCUserAggregate creates a user on the first SSO sign-in of an identity. A taken identity is
// ErrOIDCIdentityTaken: a concurrent first sign-in of the same identity won. A taken email or
// username is ErrAccountConflict, since accounts are never linked by email. The identity is claimed
// first, so a race reads as the former even though the winner holds the same email.
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
		{Scope: ClaimUserEmail, Value: u.email, Owner: u.id, Taken: taken},
		{Scope: ClaimUserUsername, Value: u.username, Owner: u.id, Taken: taken},
	}
}

func (u *UserAggregate) identityClaim(issuer, subject string) Claim {
	return Claim{Scope: ClaimOIDCIdentity, Value: OIDCIdentity(issuer, subject), Owner: u.id, Taken: ErrOIDCIdentityTaken}
}

func (u *UserAggregate) StreamID() string {
	return UserStreamID(u.id)
}

func (u *UserAggregate) ID() string {
	return u.id
}

func (u *UserAggregate) Apply(e Event) {
	switch ev := e.(type) {
	case UserRegistered:
		u.id, u.username, u.email = ev.UserID, ev.Username, ev.Email
	case UserRegisteredViaOIDC:
		u.id, u.username, u.email = ev.UserID, ev.Username, ev.Email
	case UserImported:
		u.id, u.username, u.email = ev.UserID, ev.Username, ev.Email
	}
}
