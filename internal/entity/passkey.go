package entity

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

var (
	ErrInvalidPasskeyName = newError(KindInvalid, fmt.Sprintf("passkey name must be at most %d characters", MaxPasskeyNameLen))
	ErrPasskeyNotFound    = newError(KindNotFound, "passkey not found")
	ErrPasskeyExists      = newError(KindExists, "passkey already registered")
	// ErrPasskeyCeremonyInvalid is a WebAuthn registration that cannot complete: a bad, expired or
	// foreign ceremony token, or a credential response that does not verify against it.
	ErrPasskeyCeremonyInvalid = newError(KindInvalid, "invalid passkey ceremony")
	// ErrPasskeyLoginInvalid is a passkey sign-in that cannot complete, for the same reasons.
	ErrPasskeyLoginInvalid = newError(KindUnauthenticated, "invalid passkey login")
	// ErrPasskeyChallengeUsed is a login ceremony whose challenge has already completed once: a replay.
	ErrPasskeyChallengeUsed = newError(KindConflict, "passkey login already completed")
	// ErrPasskeyReauthRequired is a request to add a passkey from a sign-in older than the allowed
	// age: the caller must sign in again first. Clients match on its message, so it must not change.
	ErrPasskeyReauthRequired = newError(KindUnauthenticated, "reauthentication required")
)

// PasskeyID is a WebAuthn credential ID: raw bytes in the database, unpadded base64url (the
// encoding WebAuthn JSON uses) in text and JSON.
type PasskeyID []byte

// Passkey is a WebAuthn credential registered to a user. Credential holds the library's full
// credential record as JSON; it never leaves the server.
type Passkey struct {
	ID         PasskeyID       `json:"id"           example:"AQIDBA"`
	UserID     string          `json:"-"`
	Name       string          `json:"name"         example:"MacBook Touch ID"`
	Credential json.RawMessage `json:"-"`
	CreatedAt  time.Time       `json:"created_at"   example:"2026-01-01T00:00:00Z"`
	LastUsedAt *time.Time      `json:"last_used_at" example:"2026-01-01T00:00:00Z"`
}

// ReadableBy is nil when userID may read the passkey: when it is theirs. Another user's passkey is
// ErrPasskeyNotFound, the same as one that does not exist.
func (p *Passkey) ReadableBy(userID string) error {
	return passkeyOwnedBy(p.UserID, userID)
}

// passkeyOwnedBy is who may read, use or change a passkey: its owner. To anyone else it does not
// exist: ErrPasskeyNotFound.
func passkeyOwnedBy(owner, userID string) error {
	if owner != userID {
		return ErrPasskeyNotFound
	}

	return nil
}

// String returns the ID as unpadded base64url.
func (id PasskeyID) String() string {
	return base64.RawURLEncoding.EncodeToString(id)
}

// MarshalText encodes the ID as unpadded base64url.
func (id PasskeyID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

// UnmarshalText decodes unpadded base64url. Text that is not base64url names no passkey, so it is
// ErrPasskeyNotFound.
func (id *PasskeyID) UnmarshalText(text []byte) error {
	decoded, err := base64.RawURLEncoding.DecodeString(string(text))
	if err != nil {
		return ErrPasskeyNotFound
	}

	*id = decoded

	return nil
}

// CeremonyStarted is a WebAuthn ceremony begun: the options for navigator.credentials, and the
// token its finish needs.
type CeremonyStarted struct {
	Options []byte
	Token   string
}

// PasskeyLogin is a verified passkey sign-in: whose it is, the credential record as the login left
// it (its new sign count), and the challenge to burn, which expires with the ceremony, so the
// sign-in completes once.
type PasskeyLogin struct {
	UserID        string
	Credential    json.RawMessage
	ChallengeHash []byte
	ExpiresAt     time.Time
}
