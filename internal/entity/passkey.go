package entity

import (
	"encoding/base64"
	"encoding/json"
	"time"
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
	PasskeyID     PasskeyID
	Credential    json.RawMessage
	ChallengeHash []byte
	ExpiresAt     time.Time
}
