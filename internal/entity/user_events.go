package entity

import "time"

// UserIdentity links a user to an identity at an OpenID Connect provider.
type UserIdentity struct {
	Issuer    string    `json:"issuer"`
	Subject   string    `json:"subject"`
	CreatedAt time.Time `json:"created_at"`
}

// UserRegistered is a user who registered with a password. PasswordHash is the bcrypt hash, never
// the password.
type UserRegistered struct {
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	At           time.Time `json:"at"`
}

// UserRegisteredViaOIDC is a user created by their first SSO sign-in: no password, linked to the
// identity that signed in.
type UserRegisteredViaOIDC struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Issuer   string    `json:"issuer"`
	Subject  string    `json:"subject"`
	At       time.Time `json:"at"`
}

// UserImported is a user written before users stored events, with the identities linked to them:
// the backfill's one event per row.
type UserImported struct {
	UserID       string         `json:"user_id"`
	Username     string         `json:"username"`
	Email        string         `json:"email"`
	PasswordHash string         `json:"password_hash"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Identities   []UserIdentity `json:"identities"`
}

// EventType -.
func (UserRegistered) EventType() string { return "UserRegistered" }

// SchemaVersion is the version of the event's shape, raised with an upcast when it changes.
func (UserRegistered) SchemaVersion() int { return 1 }

// EventType -.
func (UserRegisteredViaOIDC) EventType() string { return "UserRegisteredViaOIDC" }

// SchemaVersion -.
func (UserRegisteredViaOIDC) SchemaVersion() int { return 1 }

// EventType -.
func (UserImported) EventType() string { return "UserImported" }

// SchemaVersion -.
func (UserImported) SchemaVersion() int { return 1 }

// UserEvents lists the user events, one zero value each.
func UserEvents() []Event {
	return []Event{UserRegistered{}, UserRegisteredViaOIDC{}, UserImported{}}
}
