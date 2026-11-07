package entity

import (
	"encoding/json"
	"time"
)

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

// SchemaVersion is the version of the event's shape, raised with an upcast when it changes.
func (PasskeyRegistered) SchemaVersion() int { return 1 }

// EventType -.
func (PasskeyRenamed) EventType() string { return "PasskeyRenamed" }

// SchemaVersion -.
func (PasskeyRenamed) SchemaVersion() int { return 1 }

// EventType -.
func (PasskeyUsed) EventType() string { return "PasskeyUsed" }

// SchemaVersion -.
func (PasskeyUsed) SchemaVersion() int { return 1 }

// EventType -.
func (PasskeyDeleted) EventType() string { return "PasskeyDeleted" }

// SchemaVersion -.
func (PasskeyDeleted) SchemaVersion() int { return 1 }

// EventType -.
func (PasskeyImported) EventType() string { return "PasskeyImported" }

// SchemaVersion -.
func (PasskeyImported) SchemaVersion() int { return 1 }

// PasskeyEvents lists the passkey events, one zero value each.
func PasskeyEvents() []Event {
	return []Event{PasskeyRegistered{}, PasskeyRenamed{}, PasskeyUsed{}, PasskeyDeleted{}, PasskeyImported{}}
}
