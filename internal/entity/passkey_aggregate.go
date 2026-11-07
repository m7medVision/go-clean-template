package entity

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// _defaultPasskeyName names a passkey registered or renamed without one.
	_defaultPasskeyName = "Passkey"
	// MaxPasskeyNameLen is the longest name a passkey may have, in characters.
	MaxPasskeyNameLen = 64
	// _passkeySnapshotVersion is the version of passkeySnapshot's shape.
	_passkeySnapshotVersion = 1
)

// PasskeyStreamID is the stream of the passkey with id: one stream per credential ID, so a credential
// cannot be registered twice.
func PasskeyStreamID(id PasskeyID) string {
	return "passkey-" + id.String()
}

// passkeyName is a passkey's name trimmed, a blank one the default, and one longer than
// MaxPasskeyNameLen ErrInvalidPasskeyName.
func passkeyName(name string) (string, error) {
	name = strings.TrimSpace(name)

	switch {
	case name == "":
		return _defaultPasskeyName, nil
	case utf8.RuneCountInString(name) > MaxPasskeyNameLen:
		return "", ErrInvalidPasskeyName
	default:
		return name, nil
	}
}

// PasskeyAggregate is one WebAuthn credential on the write side. Every sign-in with it records
// PasskeyUsed, so its stream grows with use and loads from a snapshot.
type PasskeyAggregate struct {
	AggregateBase

	passkey    Passkey
	registered bool
	deleted    bool
}

var _ Snapshotter = (*PasskeyAggregate)(nil)

// ImportPasskeyAggregate records a passkey stored before passkeys stored events, as PasskeyImported.
func ImportPasskeyAggregate(row *Passkey) *PasskeyAggregate {
	a := &PasskeyAggregate{}
	Record(a, PasskeyImported{
		PasskeyID: row.ID, UserID: row.UserID, Name: row.Name, Credential: row.Credential,
		CreatedAt: row.CreatedAt, LastUsedAt: row.LastUsedAt,
	})

	return a
}

// StreamID -.
func (a *PasskeyAggregate) StreamID() string {
	return PasskeyStreamID(a.passkey.ID)
}

// State is the passkey as its events add up.
func (a *PasskeyAggregate) State() Passkey {
	return a.passkey
}

// Live reports whether the passkey is registered and not deleted: whether it can sign in.
func (a *PasskeyAggregate) Live() bool {
	return a.registered && !a.deleted
}

// Register registers credential id for userID. A credential that is registered already is
// ErrPasskeyExists; one that was deleted can be registered again.
func (a *PasskeyAggregate) Register(id PasskeyID, userID, name string, credential json.RawMessage, at time.Time) error {
	if a.Live() {
		return ErrPasskeyExists
	}

	name, err := passkeyName(name)
	if err != nil {
		return err
	}

	Record(a, PasskeyRegistered{PasskeyID: id, UserID: userID, Name: name, Credential: credential, At: at})

	return nil
}

// Rename names userID's passkey. Another user's passkey, or one that is gone, is ErrPasskeyNotFound.
func (a *PasskeyAggregate) Rename(userID, name string, at time.Time) error {
	if !a.ownedBy(userID) {
		return ErrPasskeyNotFound
	}

	name, err := passkeyName(name)
	if err != nil {
		return err
	}

	Record(a, PasskeyRenamed{PasskeyID: a.passkey.ID, Name: name, At: at})

	return nil
}

// Use records a sign-in with the passkey, and the credential record as the login left it.
func (a *PasskeyAggregate) Use(credential json.RawMessage, at time.Time) error {
	if !a.Live() {
		return ErrPasskeyNotFound
	}

	Record(a, PasskeyUsed{PasskeyID: a.passkey.ID, Credential: credential, At: at})

	return nil
}

// Delete deletes userID's passkey. Another user's passkey, or one that is gone, is ErrPasskeyNotFound.
func (a *PasskeyAggregate) Delete(userID string, at time.Time) error {
	if !a.ownedBy(userID) {
		return ErrPasskeyNotFound
	}

	Record(a, PasskeyDeleted{PasskeyID: a.passkey.ID, At: at})

	return nil
}

func (a *PasskeyAggregate) ownedBy(userID string) bool {
	return a.Live() && a.passkey.UserID == userID
}

// Apply -.
func (a *PasskeyAggregate) Apply(e Event) {
	switch ev := e.(type) {
	case PasskeyRegistered:
		a.passkey = Passkey{ID: ev.PasskeyID, UserID: ev.UserID, Name: ev.Name, Credential: ev.Credential, CreatedAt: ev.At}
		a.registered, a.deleted = true, false
	case PasskeyImported:
		a.passkey = Passkey{
			ID: ev.PasskeyID, UserID: ev.UserID, Name: ev.Name, Credential: ev.Credential,
			CreatedAt: ev.CreatedAt, LastUsedAt: ev.LastUsedAt,
		}
		a.registered, a.deleted = true, false
	case PasskeyRenamed:
		a.passkey.Name = ev.Name
	case PasskeyUsed:
		at := ev.At
		a.passkey.Credential, a.passkey.LastUsedAt = ev.Credential, &at
	case PasskeyDeleted:
		a.deleted = true
	}
}

// passkeySnapshot carries the fields Passkey's JSON leaves out, the owner and the credential.
type passkeySnapshot struct {
	Passkey    Passkey         `json:"passkey"`
	UserID     string          `json:"user_id"`
	Credential json.RawMessage `json:"credential"`
	Registered bool            `json:"registered"`
	Deleted    bool            `json:"deleted"`
}

// SnapshotVersion -.
func (*PasskeyAggregate) SnapshotVersion() int {
	return _passkeySnapshotVersion
}

// Snapshot -.
func (a *PasskeyAggregate) Snapshot() any {
	return passkeySnapshot{
		Passkey: a.passkey, UserID: a.passkey.UserID, Credential: a.passkey.Credential,
		Registered: a.registered, Deleted: a.deleted,
	}
}

// RestoreSnapshot -.
func (a *PasskeyAggregate) RestoreSnapshot(data json.RawMessage) error {
	var s passkeySnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("PasskeyAggregate - RestoreSnapshot: %w", err)
	}

	a.passkey, a.registered, a.deleted = s.Passkey, s.Registered, s.Deleted
	a.passkey.UserID, a.passkey.Credential = s.UserID, s.Credential

	return nil
}
