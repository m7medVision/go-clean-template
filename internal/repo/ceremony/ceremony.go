// Package ceremony runs the WebAuthn ceremonies of passkey registration and login, as
// repo.PasskeyCeremony. go-webauthn stays inside this package: callers pass and get the WebAuthn
// JSON the browser speaks, and a ceremony's state travels between begin and finish in a signed
// token. It stores nothing: the passkey commands and queries decide what to read and what to record.
package ceremony

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
)

// _scope scopes the ceremony token's key and audience, so no other token passes for it.
const _scope = "webauthn-ceremony"

// Ceremony begins and finishes the WebAuthn ceremonies.
type Ceremony struct {
	webauthn *webauthn.WebAuthn
	signer   *jwt.Scoped
	timeout  time.Duration
	reauth   time.Duration
	now      func() time.Time
}

var _ repo.PasskeyCeremony = (*Ceremony)(nil)

// New returns the ceremony of rp, the relying party: who passkeys are registered with, and where
// from. Its tokens are signed with a key derived from secret and last rp.CeremonyTimeout, which the
// browser is sent and finish enforces. now is the clock a sign-in's age is measured against.
// Invalid WebAuthn settings fail.
func New(secret string, rp config.WebAuthn, now func() time.Time) (*Ceremony, error) {
	signer, err := jwt.NewScoped(secret, _scope, rp.CeremonyTimeout)
	if err != nil {
		return nil, fmt.Errorf("ceremony - New - jwt.NewScoped: %w", err)
	}

	w, err := webauthn.New(&webauthn.Config{
		RPID:                  rp.RPID,
		RPDisplayName:         rp.RPName,
		RPOrigins:             rp.RPOrigins,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification:   protocol.VerificationRequired,
		},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: rp.CeremonyTimeout, TimeoutUVD: rp.CeremonyTimeout},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: rp.CeremonyTimeout, TimeoutUVD: rp.CeremonyTimeout},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ceremony - New - webauthn.New: %w", err)
	}

	return &Ceremony{webauthn: w, signer: signer, timeout: rp.CeremonyTimeout, reauth: rp.ReauthMaxAge, now: now}, nil
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
func (c *Ceremony) BeginRegistration(user *entity.User, stored []entity.Passkey) (entity.CeremonyStarted, error) {
	acc, err := newAccount(user, stored)
	if err != nil {
		return entity.CeremonyStarted{}, fmt.Errorf("ceremony - BeginRegistration - newAccount: %w", err)
	}

	creation, session, err := c.webauthn.BeginRegistration(acc,
		webauthn.WithExclusions(webauthn.Credentials(acc.credentials).CredentialDescriptors()))
	if err != nil {
		return entity.CeremonyStarted{}, fmt.Errorf("ceremony - BeginRegistration - c.webauthn.BeginRegistration: %w", err)
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
func (c *Ceremony) BeginLogin() (entity.CeremonyStarted, error) {
	assertion, session, err := c.webauthn.BeginDiscoverableLogin()
	if err != nil {
		return entity.CeremonyStarted{}, fmt.Errorf("ceremony - BeginLogin - c.webauthn.BeginDiscoverableLogin: %w", err)
	}

	return c.started(assertion.Response, session)
}

func (c *Ceremony) started(options any, session *webauthn.SessionData) (entity.CeremonyStarted, error) {
	encoded, err := json.Marshal(options)
	if err != nil {
		return entity.CeremonyStarted{}, fmt.Errorf("ceremony - json.Marshal options: %w", err)
	}

	token, err := c.signSession(session)
	if err != nil {
		return entity.CeremonyStarted{}, err
	}

	return entity.CeremonyStarted{Options: encoded, Token: token}, nil
}

// LoginFor reads the user and passkey an assertion (PublicKeyCredential.toJSON) names: the user
// handle is the user's UUID bytes, the raw ID the credential's. Nothing is verified yet.
func (c *Ceremony) LoginFor(body []byte) (userID string, id entity.PasskeyID, err error) {
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
	if err != nil {
		return "", nil, fmt.Errorf("ceremony - LoginFor - protocol.ParseCredentialRequestResponseBody: %w: %w",
			entity.ErrPasskeyCeremonyInvalid, err)
	}

	owner, err := uuid.FromBytes(parsed.Response.UserHandle)
	if err != nil {
		return "", nil, fmt.Errorf("ceremony - LoginFor - uuid.FromBytes: %w: %w", entity.ErrPasskeyCeremonyInvalid, err)
	}

	return owner.String(), parsed.RawID, nil
}

// FinishLogin verifies the browser's assertion (PublicKeyCredential.toJSON) against the ceremony in
// token, for user's passkey as stored, and returns the sign-in to record. It rejects a credential
// whose non-zero sign count did not advance (a possible clone).
func (c *Ceremony) FinishLogin(
	token string, body []byte, user *entity.User, passkey *entity.Passkey,
) (entity.PasskeyLogin, error) {
	session, err := c.verifySession(token)
	if err != nil {
		return entity.PasskeyLogin{}, fmt.Errorf("ceremony - FinishLogin - c.verifySession: %w", err)
	}

	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
	if err != nil {
		return entity.PasskeyLogin{}, fmt.Errorf("ceremony - FinishLogin - protocol.ParseCredentialRequestResponseBody: %w: %w",
			entity.ErrPasskeyCeremonyInvalid, err)
	}

	if !bytes.Equal(parsed.RawID, passkey.ID) {
		return entity.PasskeyLogin{}, fmt.Errorf("ceremony - FinishLogin - the assertion is another passkey's: %w",
			entity.ErrPasskeyCeremonyInvalid)
	}

	owner, err := newAccount(user, []entity.Passkey{*passkey})
	if err != nil {
		return entity.PasskeyLogin{}, fmt.Errorf("ceremony - FinishLogin - newAccount: %w", err)
	}

	found := func([]byte, []byte) (webauthn.User, error) { return owner, nil }

	_, credential, err := c.webauthn.ValidatePasskeyLogin(found, session, parsed)
	if err != nil {
		return entity.PasskeyLogin{}, fmt.Errorf("ceremony - FinishLogin - c.webauthn.ValidatePasskeyLogin: %w: %w",
			entity.ErrPasskeyCeremonyInvalid, err)
	}

	if credential.Authenticator.CloneWarning {
		return entity.PasskeyLogin{}, fmt.Errorf("ceremony - FinishLogin - sign count did not advance: %w",
			entity.ErrPasskeyCeremonyInvalid)
	}

	record, err := json.Marshal(credential)
	if err != nil {
		return entity.PasskeyLogin{}, fmt.Errorf("ceremony - FinishLogin - json.Marshal credential: %w", err)
	}

	expiresAt := session.Expires
	if expiresAt.IsZero() {
		expiresAt = c.now().Add(c.timeout)
	}

	hash := sha256.Sum256([]byte(session.Challenge))

	return entity.PasskeyLogin{
		UserID: owner.id, PasskeyID: credential.ID, Credential: record, ChallengeHash: hash[:], ExpiresAt: expiresAt.UTC(),
	}, nil
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
