package entity

import (
	"encoding/json"
	"time"
)

// The passkey events, all at schema version 1. Credential is the WebAuthn library's credential
// record as JSON: the public key, sign count and flags. It never leaves the server.
const _passkeyEventsVersion = 1

// PasskeyRegistered is a WebAuthn credential a user registered.
type PasskeyRegistered struct {
	PasskeyID  PasskeyID       `json:"passkey_id"`
	UserID     string          `json:"user_id"`
	Name       string          `json:"name"`
	Credential json.RawMessage `json:"credential"`
	At         time.Time       `json:"at"`
}

// PasskeyRenamed gives a passkey a new name.
type PasskeyRenamed struct {
	PasskeyID PasskeyID `json:"passkey_id"`
	Name      string    `json:"name"`
	At        time.Time `json:"at"`
}

// PasskeyUsed is a sign-in with a passkey: the credential record as the login left it (its new sign
// count), and when.
type PasskeyUsed struct {
	PasskeyID  PasskeyID       `json:"passkey_id"`
	Credential json.RawMessage `json:"credential"`
	At         time.Time       `json:"at"`
}

// PasskeyDeleted removes a passkey: it can no longer sign in.
type PasskeyDeleted struct {
	PasskeyID PasskeyID `json:"passkey_id"`
	At        time.Time `json:"at"`
}

// PasskeyImported is a passkey stored before passkeys stored events: the backfill's one event per
// webauthn_credentials row.
type PasskeyImported struct {
	PasskeyID  PasskeyID       `json:"passkey_id"`
	UserID     string          `json:"user_id"`
	Name       string          `json:"name"`
	Credential json.RawMessage `json:"credential"`
	CreatedAt  time.Time       `json:"created_at"`
	LastUsedAt *time.Time      `json:"last_used_at"`
}

// EventType -.
func (PasskeyRegistered) EventType() string { return "PasskeyRegistered" }

// SchemaVersion -.
func (PasskeyRegistered) SchemaVersion() int { return _passkeyEventsVersion }

// EventType -.
func (PasskeyRenamed) EventType() string { return "PasskeyRenamed" }

// SchemaVersion -.
func (PasskeyRenamed) SchemaVersion() int { return _passkeyEventsVersion }

// EventType -.
func (PasskeyUsed) EventType() string { return "PasskeyUsed" }

// SchemaVersion -.
func (PasskeyUsed) SchemaVersion() int { return _passkeyEventsVersion }

// EventType -.
func (PasskeyDeleted) EventType() string { return "PasskeyDeleted" }

// SchemaVersion -.
func (PasskeyDeleted) SchemaVersion() int { return _passkeyEventsVersion }

// EventType -.
func (PasskeyImported) EventType() string { return "PasskeyImported" }

// SchemaVersion -.
func (PasskeyImported) SchemaVersion() int { return _passkeyEventsVersion }

func passkeyEvents() []Event {
	return []Event{PasskeyRegistered{}, PasskeyRenamed{}, PasskeyUsed{}, PasskeyDeleted{}, PasskeyImported{}}
}
