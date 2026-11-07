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
	MaxPasskeyNameLen       = 64
	_passkeySnapshotVersion = 1
)

// PasskeyStreamID is the stream of the passkey with id: one stream per credential ID, so a credential
// cannot be registered twice.
func PasskeyStreamID(id PasskeyID) string {
	return "passkey-" + id.String()
}

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

	passkey    passkeyState
	registered bool
	deleted    bool
}

// passkeyState is what the passkey's rules and a sign-in need: who owns it, and the credential
// record the next sign-in verifies against.
type passkeyState struct {
	ID         PasskeyID
	UserID     string
	Credential json.RawMessage
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

func (a *PasskeyAggregate) StreamID() string {
	return PasskeyStreamID(a.passkey.ID)
}

// ID is the passkey's credential ID, unpadded base64url.
func (a *PasskeyAggregate) ID() string {
	return a.passkey.ID.String()
}

// StreamOf names the stream of the passkey whose ID is id, unpadded base64url. An id that is not
// base64url names no passkey: ErrPasskeyNotFound.
func (*PasskeyAggregate) StreamOf(id string) (string, error) {
	var pid PasskeyID
	if err := pid.UnmarshalText([]byte(id)); err != nil {
		return "", err
	}

	return PasskeyStreamID(pid), nil
}

// NotFound is the error for a passkey ID with no stream.
func (*PasskeyAggregate) NotFound() error {
	return ErrPasskeyNotFound
}

// Credential is the credential record the next sign-in verifies against.
func (a *PasskeyAggregate) Credential() json.RawMessage {
	return a.passkey.Credential
}

// OwnedBy is nil when userID may use, rename or delete the passkey: when it is live and theirs, as
// a read of it is. Another user's passkey, or one that is gone, is ErrPasskeyNotFound.
func (a *PasskeyAggregate) OwnedBy(userID string) error {
	if !a.Live() {
		return ErrPasskeyNotFound
	}

	return passkeyOwnedBy(a.passkey.UserID, userID)
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
	if err := a.OwnedBy(userID); err != nil {
		return err
	}

	name, err := passkeyName(name)
	if err != nil {
		return err
	}

	Record(a, PasskeyRenamed{PasskeyID: a.passkey.ID, Name: name, At: at})

	return nil
}

// Use records userID's sign-in with the passkey, and the credential record as the login left it.
// Another user's passkey, or one that is gone, is ErrPasskeyNotFound.
func (a *PasskeyAggregate) Use(userID string, credential json.RawMessage, at time.Time) error {
	if err := a.OwnedBy(userID); err != nil {
		return err
	}

	Record(a, PasskeyUsed{PasskeyID: a.passkey.ID, Credential: credential, At: at})

	return nil
}

// Delete deletes userID's passkey. Another user's passkey, or one that is gone, is ErrPasskeyNotFound.
func (a *PasskeyAggregate) Delete(userID string, at time.Time) error {
	if err := a.OwnedBy(userID); err != nil {
		return err
	}

	Record(a, PasskeyDeleted{PasskeyID: a.passkey.ID, At: at})

	return nil
}

func (a *PasskeyAggregate) Apply(e Event) {
	switch ev := e.(type) {
	case PasskeyRegistered:
		a.passkey = passkeyState{ID: ev.PasskeyID, UserID: ev.UserID, Credential: ev.Credential}
		a.registered, a.deleted = true, false
	case PasskeyImported:
		a.passkey = passkeyState{ID: ev.PasskeyID, UserID: ev.UserID, Credential: ev.Credential}
		a.registered, a.deleted = true, false
	case PasskeyRenamed: // the name is the read model's; no rule reads it
	case PasskeyUsed:
		a.passkey.Credential = ev.Credential
	case PasskeyDeleted:
		a.deleted = true
	}
}

// passkeySnapshot is the passkey's state. Its shape is the one snapshots were first stored in, the
// passkey's ID nested under "passkey", so those still load.
type passkeySnapshot struct {
	Passkey struct {
		ID PasskeyID `json:"id"`
	} `json:"passkey"`
	UserID     string          `json:"user_id"`
	Credential json.RawMessage `json:"credential"`
	Registered bool            `json:"registered"`
	Deleted    bool            `json:"deleted"`
}

func (*PasskeyAggregate) SnapshotVersion() int {
	return _passkeySnapshotVersion
}

func (a *PasskeyAggregate) Snapshot() any {
	s := passkeySnapshot{
		UserID: a.passkey.UserID, Credential: a.passkey.Credential, Registered: a.registered, Deleted: a.deleted,
	}
	s.Passkey.ID = a.passkey.ID

	return s
}

func (a *PasskeyAggregate) RestoreSnapshot(data json.RawMessage) error {
	var s passkeySnapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("PasskeyAggregate - RestoreSnapshot: %w", err)
	}

	a.passkey = passkeyState{ID: s.Passkey.ID, UserID: s.UserID, Credential: s.Credential}
	a.registered, a.deleted = s.Registered, s.Deleted

	return nil
}
