// Package ceremony runs the WebAuthn ceremonies of passkey registration and login. go-webauthn
// stays inside this package: callers pass and get the WebAuthn JSON the browser speaks, and a
// ceremony's state travels between begin and finish in a signed token. It stores nothing: the passkey
// commands and queries decide what to read and what to record.
package ceremony

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

// Config is the relying party: who passkeys are registered with, and where from.
type Config struct {
	RPID      string
	RPName    string
	RPOrigins []string
	// Timeout is the ceremony's lifetime, sent to the browser and enforced on finish.
	Timeout time.Duration
	// ReauthMaxAge is how recent a sign-in must be to add a passkey.
	ReauthMaxAge time.Duration
	// Now is the clock the sign-in age is measured against; nil is time.Now.
	Now func() time.Time
}

// Ceremony begins and finishes the WebAuthn ceremonies.
type Ceremony struct {
	webauthn *webauthn.WebAuthn
	signer   *jwt.Scoped
	timeout  time.Duration
	reauth   time.Duration
	now      func() time.Time
}

// Started is a ceremony begun: the options for navigator.credentials, and the token its finish
// needs.
type Started struct {
	Options []byte
	Token   string
}

// Login is a verified passkey sign-in: whose it is, the credential record as the login left it (its
// new sign count), and the challenge to burn so the sign-in completes once.
type Login struct {
	UserID        string
	PasskeyID     entity.PasskeyID
	Credential    json.RawMessage
	ChallengeHash []byte
	ExpiresAt     time.Time
}

// Lookup finds the user a login's handle names and their passkey with the credential ID the
// assertion carries. An unknown user is entity.ErrUserNotFound; an unknown passkey, or one that is
// not the user's, is nil.
type Lookup func(userID string, id entity.PasskeyID) (*entity.User, *entity.Passkey, error)

// New returns the ceremony of cfg's relying party. signer seals the state a ceremony carries from
// begin to finish; its lifetime should match cfg.Timeout.
func New(signer *jwt.Scoped, cfg *Config) (*Ceremony, error) {
	w, err := webauthn.New(&webauthn.Config{
		RPID:                  cfg.RPID,
		RPDisplayName:         cfg.RPName,
		RPOrigins:             cfg.RPOrigins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification:   protocol.VerificationRequired,
		},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: cfg.Timeout, TimeoutUVD: cfg.Timeout},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: cfg.Timeout, TimeoutUVD: cfg.Timeout},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ceremony - New - webauthn.New: %w", err)
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return &Ceremony{webauthn: w, signer: signer, timeout: cfg.Timeout, reauth: cfg.ReauthMaxAge, now: now}, nil
}

// CheckReauth refuses a sign-in too old to add a passkey. A passkey outlives the session that adds
// it and any password change, so adding one needs a recent sign-in: authTime, when the caller's auth
// token was issued, must be within the reauth max age. A stolen token past that age cannot plant a
// passkey. A zero authTime (a token without one) is too old.
func (c *Ceremony) CheckReauth(authTime time.Time) error {
	if authTime.IsZero() || c.now().Sub(authTime) > c.reauth {
		return fmt.Errorf("signed in at %s: %w", authTime.UTC().Format(time.RFC3339), entity.ErrPasskeyReauthRequired)
	}

	return nil
}

// BeginRegistration returns the creation options for navigator.credentials.create, excluding the
// user's stored passkeys, and the token FinishRegistration needs.
func (c *Ceremony) BeginRegistration(user *entity.User, stored []entity.Passkey) (Started, error) {
	acc, err := newAccount(user, stored)
	if err != nil {
		return Started{}, fmt.Errorf("ceremony - BeginRegistration - newAccount: %w", err)
	}

	creation, session, err := c.webauthn.BeginRegistration(acc,
		webauthn.WithExclusions(webauthn.Credentials(acc.credentials).CredentialDescriptors()))
	if err != nil {
		return Started{}, fmt.Errorf("ceremony - BeginRegistration - c.webauthn.BeginRegistration: %w", err)
	}

	return c.started(creation.Response, session)
}

// FinishRegistration verifies the browser's credential (PublicKeyCredential.toJSON) against the
// ceremony in token, begun for userID, and returns the credential's ID and record to store. A token
// or body that does not complete the ceremony is entity.ErrPasskeyCeremonyInvalid.
func (c *Ceremony) FinishRegistration(userID, token string, body []byte) (entity.PasskeyID, json.RawMessage, error) {
	session, err := c.verifySession(token)
	if err != nil {
		return nil, nil, fmt.Errorf("ceremony - FinishRegistration - c.verifySession: %w", err)
	}

	handle, err := userHandle(userID)
	if err != nil {
		return nil, nil, fmt.Errorf("ceremony - FinishRegistration - userHandle: %w", err)
	}

	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("ceremony - FinishRegistration - protocol.ParseCredentialCreationResponseBody: %w: %w",
			entity.ErrPasskeyCeremonyInvalid, err)
	}

	// The account's handle must match the one the session was begun for; CreateCredential checks it.
	credential, err := c.webauthn.CreateCredential(&account{handle: handle}, session, parsed)
	if err != nil {
		return nil, nil, fmt.Errorf("ceremony - FinishRegistration - c.webauthn.CreateCredential: %w: %w",
			entity.ErrPasskeyCeremonyInvalid, err)
	}

	record, err := json.Marshal(credential)
	if err != nil {
		return nil, nil, fmt.Errorf("ceremony - FinishRegistration - json.Marshal credential: %w", err)
	}

	return credential.ID, record, nil
}

// BeginLogin returns the request options for a discoverable navigator.credentials.get (no
// allowCredentials: the authenticator offers its passkeys for this RP) and the token FinishLogin
// needs.
func (c *Ceremony) BeginLogin() (Started, error) {
	assertion, session, err := c.webauthn.BeginDiscoverableLogin()
	if err != nil {
		return Started{}, fmt.Errorf("ceremony - BeginLogin - c.webauthn.BeginDiscoverableLogin: %w", err)
	}

	return c.started(assertion.Response, session)
}

func (c *Ceremony) started(options any, session *webauthn.SessionData) (Started, error) {
	encoded, err := json.Marshal(options)
	if err != nil {
		return Started{}, fmt.Errorf("ceremony - json.Marshal options: %w", err)
	}

	token, err := c.signSession(session)
	if err != nil {
		return Started{}, err
	}

	return Started{Options: encoded, Token: token}, nil
}

// FinishLogin verifies the browser's assertion (PublicKeyCredential.toJSON) against the ceremony in
// token, for the account lookup finds, and returns the sign-in to record. It rejects a credential
// whose non-zero sign count did not advance (a possible clone). Anything that does not complete the
// ceremony is entity.ErrPasskeyCeremonyInvalid; a lookup that fails for another reason is returned as
// is.
func (c *Ceremony) FinishLogin(token string, body []byte, lookup Lookup) (Login, error) {
	session, err := c.verifySession(token)
	if err != nil {
		return Login{}, fmt.Errorf("ceremony - FinishLogin - c.verifySession: %w", err)
	}

	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
	if err != nil {
		return Login{}, fmt.Errorf("ceremony - FinishLogin - protocol.ParseCredentialRequestResponseBody: %w: %w",
			entity.ErrPasskeyCeremonyInvalid, err)
	}

	owner, credential, err := c.validateLogin(&session, parsed, lookup)
	if err != nil {
		return Login{}, fmt.Errorf("ceremony - FinishLogin: %w", err)
	}

	record, err := json.Marshal(credential)
	if err != nil {
		return Login{}, fmt.Errorf("ceremony - FinishLogin - json.Marshal credential: %w", err)
	}

	expiresAt := session.Expires
	if expiresAt.IsZero() {
		expiresAt = c.now().Add(c.timeout)
	}

	hash := sha256.Sum256([]byte(session.Challenge))

	return Login{
		UserID: owner.id, PasskeyID: credential.ID, Credential: record, ChallengeHash: hash[:], ExpiresAt: expiresAt.UTC(),
	}, nil
}

func (c *Ceremony) validateLogin(
	session *webauthn.SessionData, parsed *protocol.ParsedCredentialAssertionData, lookup Lookup,
) (*account, *webauthn.Credential, error) {
	var lookupErr error

	handler := func(rawID, handle []byte) (webauthn.User, error) {
		owner, err := lookupAccount(rawID, handle, lookup)
		if err != nil {
			lookupErr = err

			return nil, err
		}

		return owner, nil
	}

	user, credential, err := c.webauthn.ValidatePasskeyLogin(handler, *session, parsed)

	if lookupErr != nil && !errors.Is(lookupErr, entity.ErrPasskeyCeremonyInvalid) {
		return nil, nil, lookupErr
	}

	if err != nil {
		return nil, nil, fmt.Errorf("c.webauthn.ValidatePasskeyLogin: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	if credential.Authenticator.CloneWarning {
		return nil, nil, fmt.Errorf("sign count did not advance: %w", entity.ErrPasskeyCeremonyInvalid)
	}

	owner, ok := user.(*account)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected user %T: %w", user, entity.ErrPasskeyCeremonyInvalid)
	}

	return owner, credential, nil
}

// lookupAccount is the discoverable login handler: the user handle is the owner's UUID bytes. A
// handle that names no user is entity.ErrPasskeyCeremonyInvalid.
func lookupAccount(rawID, handle []byte, lookup Lookup) (*account, error) {
	id, err := uuid.FromBytes(handle)
	if err != nil {
		return nil, fmt.Errorf("uuid.FromBytes: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	user, passkey, err := lookup(id.String(), rawID)
	if err != nil {
		if errors.Is(err, entity.ErrUserNotFound) {
			return nil, fmt.Errorf("lookup: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
		}

		return nil, fmt.Errorf("lookup: %w", err)
	}

	var stored []entity.Passkey
	if passkey != nil {
		stored = []entity.Passkey{*passkey}
	}

	return newAccount(user, stored)
}

// signSession seals the ceremony state into a token the client hands back on finish.
func (c *Ceremony) signSession(session *webauthn.SessionData) (string, error) {
	payload, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("ceremony - json.Marshal session: %w", err)
	}

	token, err := c.signer.Sign(payload)
	if err != nil {
		return "", fmt.Errorf("ceremony - c.signer.Sign: %w", err)
	}

	return token, nil
}

// verifySession opens a token from signSession. A token that is forged, expired, or not a ceremony
// token is entity.ErrPasskeyCeremonyInvalid.
func (c *Ceremony) verifySession(token string) (webauthn.SessionData, error) {
	payload, err := c.signer.Verify(token)
	if err != nil {
		return webauthn.SessionData{}, fmt.Errorf("c.signer.Verify: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	var session webauthn.SessionData

	err = json.Unmarshal(payload, &session)
	if err != nil {
		return webauthn.SessionData{}, fmt.Errorf("json.Unmarshal: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	return session, nil
}

// userHandle is the WebAuthn user handle for a user: their UUID's 16 bytes. An ID that is not a
// UUID names no account a ceremony could belong to.
func userHandle(userID string) ([]byte, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("uuid.Parse: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	return id[:], nil
}

// account adapts a user and their stored passkeys to webauthn.User.
type account struct {
	id          string
	handle      []byte
	name        string
	displayName string
	credentials []webauthn.Credential
}

var _ webauthn.User = (*account)(nil)

func newAccount(user *entity.User, stored []entity.Passkey) (*account, error) {
	handle, err := userHandle(user.ID)
	if err != nil {
		return nil, err
	}

	credentials := make([]webauthn.Credential, len(stored))
	for i := range stored {
		err = json.Unmarshal(stored[i].Credential, &credentials[i])
		if err != nil {
			return nil, fmt.Errorf("json.Unmarshal passkey %s: %w", stored[i].ID, err)
		}
	}

	return &account{
		id:          user.ID,
		handle:      handle,
		name:        user.Email,
		displayName: user.Username,
		credentials: credentials,
	}, nil
}

func (a *account) WebAuthnID() []byte                         { return a.handle }
func (a *account) WebAuthnName() string                       { return a.name }
func (a *account) WebAuthnDisplayName() string                { return a.displayName }
func (a *account) WebAuthnCredentials() []webauthn.Credential { return a.credentials }
