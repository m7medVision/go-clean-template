package ceremony_test

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/descope/virtualwebauthn"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/ceremony"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/passkeytest"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errLookup = errors.New("lookup failed")

// newCeremony returns a ceremony whose tokens live for ttl, on clock now (nil: the real one).
func newCeremony(t *testing.T, ttl time.Duration, now func() time.Time) *ceremony.Ceremony {
	t.Helper()

	signer, err := jwt.NewScoped(passkeytest.SigningKey, passkeytest.CeremonyPurpose, ttl)
	require.NoError(t, err)

	c, err := ceremony.New(signer, &ceremony.Config{
		RPID:         passkeytest.RPID,
		RPName:       passkeytest.RPName,
		RPOrigins:    []string{passkeytest.RPOrigin},
		Timeout:      passkeytest.CeremonyTimeout,
		ReauthMaxAge: passkeytest.ReauthMaxAge,
		Now:          now,
	})
	require.NoError(t, err)

	return c
}

func newUser() entity.User {
	return entity.User{ID: uuid.NewString(), Username: "alice", Email: "alice@example.com"}
}

func TestRegistrationReturnsTheCredentialToStore(t *testing.T) {
	t.Parallel()

	c := newCeremony(t, passkeytest.CeremonyTimeout, nil)
	user := newUser()

	started, err := c.BeginRegistration(&user, nil)
	require.NoError(t, err)
	require.NotEmpty(t, started.Token)

	cred := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)

	id, record, err := c.FinishRegistration(user.ID, started.Token, passkeytest.Attest(t, started.Options, cred))
	require.NoError(t, err)
	assert.Equal(t, entity.PasskeyID(cred.ID), id)

	var stored webauthn.Credential

	require.NoError(t, json.Unmarshal(record, &stored))
	assert.Equal(t, cred.ID, stored.ID)
	assert.NotEmpty(t, stored.PublicKey)
}

func TestRegistrationOptions(t *testing.T) {
	t.Parallel()

	c := newCeremony(t, passkeytest.CeremonyTimeout, nil)
	user := newUser()
	existing, err := json.Marshal(webauthn.Credential{ID: []byte{1, 2, 3, 4}})
	require.NoError(t, err)

	started, err := c.BeginRegistration(&user, []entity.Passkey{{ID: entity.PasskeyID{1, 2, 3, 4}, Credential: existing}})
	require.NoError(t, err)

	parsed, err := virtualwebauthn.ParseAttestationOptions(string(started.Options))
	require.NoError(t, err)
	assert.Equal(t, passkeytest.RPID, parsed.RelyingPartyID)
	assert.Equal(t, user.Email, parsed.UserName)
	assert.Equal(t, user.Username, parsed.UserDisplayName)
	assert.Equal(t, []string{"AQIDBA"}, parsed.ExcludeCredentials)

	handle := uuid.MustParse(user.ID)
	assert.Equal(t, string(handle[:]), parsed.UserID)

	var raw struct {
		Attestation            string `json:"attestation"`
		Timeout                int64  `json:"timeout"`
		AuthenticatorSelection struct {
			ResidentKey      string `json:"residentKey"`
			RequireResident  bool   `json:"requireResidentKey"`
			UserVerification string `json:"userVerification"`
		} `json:"authenticatorSelection"`
	}

	require.NoError(t, json.Unmarshal(started.Options, &raw))
	assert.Equal(t, "none", raw.Attestation)
	assert.Equal(t, passkeytest.CeremonyTimeout.Milliseconds(), raw.Timeout)
	assert.Equal(t, "required", raw.AuthenticatorSelection.ResidentKey)
	assert.True(t, raw.AuthenticatorSelection.RequireResident)
	assert.Equal(t, "required", raw.AuthenticatorSelection.UserVerification)
}

func TestAStoredCredentialThatDoesNotDecodeFailsRegistration(t *testing.T) {
	t.Parallel()

	c := newCeremony(t, passkeytest.CeremonyTimeout, nil)
	user := newUser()

	_, err := c.BeginRegistration(&user, []entity.Passkey{{ID: entity.PasskeyID{1}, Credential: json.RawMessage(`"x"`)}})

	require.Error(t, err)
}

func TestCheckReauth(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	c := newCeremony(t, passkeytest.CeremonyTimeout, func() time.Time { return now })

	for _, tt := range []struct {
		name     string
		authTime time.Time
		wantErr  error
	}{
		{name: "signed in just now", authTime: now},
		{name: "signed in exactly the max age ago", authTime: now.Add(-passkeytest.ReauthMaxAge)},
		{
			name: "signed in longer ago", authTime: now.Add(-passkeytest.ReauthMaxAge - time.Second),
			wantErr: entity.ErrPasskeyReauthRequired,
		},
		{name: "token without an issue time", authTime: time.Time{}, wantErr: entity.ErrPasskeyReauthRequired},
	} {
		err := c.CheckReauth(tt.authTime)
		if tt.wantErr != nil {
			require.ErrorIs(t, err, tt.wantErr, tt.name)

			continue
		}

		require.NoError(t, err, tt.name)
	}
}

// registrationCase finishes a begun registration with arguments that must fail as
// ErrPasskeyCeremonyInvalid.
type registrationCase struct {
	name   string
	ttl    time.Duration
	finish func(t *testing.T, c *ceremony.Ceremony, user *entity.User, started ceremony.Started) (string, string, []byte)
}

func newCred() virtualwebauthn.Credential {
	return virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)
}

func registrationInvalidCases() []registrationCase {
	return []registrationCase{
		{
			name: "tampered token", ttl: passkeytest.CeremonyTimeout,
			finish: func(t *testing.T, _ *ceremony.Ceremony, user *entity.User, s ceremony.Started) (string, string, []byte) {
				t.Helper()

				return user.ID, passkeytest.Tamper(t, s.Token), passkeytest.Attest(t, s.Options, newCred())
			},
		},
		{
			name: "expired token", ttl: -time.Minute,
			finish: func(t *testing.T, _ *ceremony.Ceremony, user *entity.User, s ceremony.Started) (string, string, []byte) {
				t.Helper()

				return user.ID, s.Token, passkeytest.Attest(t, s.Options, newCred())
			},
		},
		{
			name: "token for another user", ttl: passkeytest.CeremonyTimeout,
			finish: func(t *testing.T, _ *ceremony.Ceremony, _ *entity.User, s ceremony.Started) (string, string, []byte) {
				t.Helper()

				return uuid.NewString(), s.Token, passkeytest.Attest(t, s.Options, newCred())
			},
		},
		{
			name: "attestation for another ceremony's challenge", ttl: passkeytest.CeremonyTimeout,
			finish: func(t *testing.T, c *ceremony.Ceremony, user *entity.User, s ceremony.Started) (string, string, []byte) {
				t.Helper()

				other, err := c.BeginRegistration(user, nil)
				require.NoError(t, err)

				return user.ID, other.Token, passkeytest.Attest(t, s.Options, newCred())
			},
		},
		{
			name: "body that is not a credential", ttl: passkeytest.CeremonyTimeout,
			finish: func(t *testing.T, _ *ceremony.Ceremony, user *entity.User, s ceremony.Started) (string, string, []byte) {
				t.Helper()

				return user.ID, s.Token, []byte(`{"id":"nope"}`)
			},
		},
		{
			name: "attestation without user verification", ttl: passkeytest.CeremonyTimeout,
			finish: func(t *testing.T, _ *ceremony.Ceremony, user *entity.User, s ceremony.Started) (string, string, []byte) {
				t.Helper()

				unverified := virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{UserNotVerified: true})

				return user.ID, s.Token, passkeytest.AttestWith(t, s.Options, newCred(), &unverified)
			},
		},
		{
			name: "user ID that is not a UUID", ttl: passkeytest.CeremonyTimeout,
			finish: func(t *testing.T, _ *ceremony.Ceremony, _ *entity.User, s ceremony.Started) (string, string, []byte) {
				t.Helper()

				return "not-a-uuid", s.Token, passkeytest.Attest(t, s.Options, newCred())
			},
		},
	}
}

func TestFinishRegistrationRefusesAnInvalidCeremony(t *testing.T) {
	t.Parallel()

	for _, tc := range registrationInvalidCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := newCeremony(t, tc.ttl, nil)
			user := newUser()

			started, err := c.BeginRegistration(&user, nil)
			require.NoError(t, err)

			userID, token, body := tc.finish(t, c, &user, started)

			_, _, err = c.FinishRegistration(userID, token, body)
			require.ErrorIs(t, err, entity.ErrPasskeyCeremonyInvalid)
		})
	}
}

// account is a user with a passkey registered through the real ceremony, the record stored for it,
// and the virtual authenticator holding it.
type account struct {
	user    entity.User
	passkey entity.Passkey
	cred    virtualwebauthn.Credential
	auth    virtualwebauthn.Authenticator
}

func register(t *testing.T, c *ceremony.Ceremony) *account {
	t.Helper()

	user := newUser()

	started, err := c.BeginRegistration(&user, nil)
	require.NoError(t, err)

	cred := newCred()

	id, record, err := c.FinishRegistration(user.ID, started.Token, passkeytest.Attest(t, started.Options, cred))
	require.NoError(t, err)

	handle := uuid.MustParse(user.ID)

	return &account{
		user:    user,
		passkey: entity.Passkey{ID: id, UserID: user.ID, Name: "Laptop", Credential: record},
		cred:    cred,
		auth:    virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{UserHandle: handle[:]}),
	}
}

// lookup finds acc by its user and credential, as the login handler's would.
func (acc *account) lookup(userID string, id entity.PasskeyID) (*entity.User, *entity.Passkey, error) {
	if userID != acc.user.ID {
		return nil, nil, entity.ErrUserNotFound
	}

	if string(id) != string(acc.passkey.ID) {
		return &acc.user, nil, nil
	}

	return &acc.user, &acc.passkey, nil
}

func (acc *account) assert(t *testing.T, options []byte) []byte {
	t.Helper()

	return passkeytest.Assert(t, options, acc.cred, &acc.auth)
}

func loginChallenge(t *testing.T, options []byte) string {
	t.Helper()

	var raw struct {
		Challenge        string `json:"challenge"`
		RPID             string `json:"rpId"`
		Timeout          int64  `json:"timeout"`
		UserVerification string `json:"userVerification"`
	}

	require.NoError(t, json.Unmarshal(options, &raw))
	assert.Equal(t, passkeytest.RPID, raw.RPID)
	assert.Equal(t, passkeytest.CeremonyTimeout.Milliseconds(), raw.Timeout)
	assert.Equal(t, "required", raw.UserVerification)
	require.NotEmpty(t, raw.Challenge)

	return raw.Challenge
}

func TestLoginReturnsTheSignInToRecord(t *testing.T) {
	t.Parallel()

	c := newCeremony(t, passkeytest.CeremonyTimeout, nil)
	acc := register(t, c)
	acc.cred.Counter = 1

	started, err := c.BeginLogin()
	require.NoError(t, err)

	challenge := sha256.Sum256([]byte(loginChallenge(t, started.Options)))
	before := time.Now()

	login, err := c.FinishLogin(started.Token, acc.assert(t, started.Options), acc.lookup)
	require.NoError(t, err)

	assert.Equal(t, acc.user.ID, login.UserID)
	assert.Equal(t, acc.passkey.ID, login.PasskeyID)
	assert.Equal(t, challenge[:], login.ChallengeHash, "the burned hash is the challenge's SHA-256")
	assert.WithinDuration(t, before.Add(passkeytest.CeremonyTimeout), login.ExpiresAt, time.Minute)

	var record webauthn.Credential

	require.NoError(t, json.Unmarshal(login.Credential, &record))
	assert.Equal(t, uint32(1), record.Authenticator.SignCount)
	assert.False(t, record.Authenticator.CloneWarning)
}

func TestAnAuthenticatorWithoutACounterKeepsSigningIn(t *testing.T) {
	t.Parallel()

	c := newCeremony(t, passkeytest.CeremonyTimeout, nil)
	acc := register(t, c)

	started, err := c.BeginLogin()
	require.NoError(t, err)

	_, err = c.FinishLogin(started.Token, acc.assert(t, started.Options), acc.lookup)
	require.NoError(t, err)
}

func TestALookupFailureIsNotABadCeremony(t *testing.T) {
	t.Parallel()

	c := newCeremony(t, passkeytest.CeremonyTimeout, nil)
	acc := register(t, c)

	started, err := c.BeginLogin()
	require.NoError(t, err)

	_, err = c.FinishLogin(started.Token, acc.assert(t, started.Options),
		func(string, entity.PasskeyID) (*entity.User, *entity.Passkey, error) { return nil, nil, errLookup })

	require.ErrorIs(t, err, errLookup)
	require.NotErrorIs(t, err, entity.ErrPasskeyCeremonyInvalid)
}

// loginCase finishes a begun login with arguments that must fail as ErrPasskeyCeremonyInvalid.
type loginCase struct {
	name    string
	ttl     time.Duration
	prepare func(t *testing.T, c *ceremony.Ceremony, acc *account, started ceremony.Started) (string, []byte)
}

func loginTokenCases() []loginCase {
	return []loginCase{
		{
			name: "tampered token", ttl: passkeytest.CeremonyTimeout,
			prepare: func(t *testing.T, _ *ceremony.Ceremony, acc *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				return passkeytest.Tamper(t, s.Token), acc.assert(t, s.Options)
			},
		},
		{
			name: "expired token", ttl: -time.Minute,
			prepare: func(t *testing.T, _ *ceremony.Ceremony, acc *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				return s.Token, acc.assert(t, s.Options)
			},
		},
		{
			name: "registration token", ttl: passkeytest.CeremonyTimeout,
			prepare: func(t *testing.T, c *ceremony.Ceremony, acc *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				other, err := c.BeginRegistration(&acc.user, nil)
				require.NoError(t, err)

				return other.Token, acc.assert(t, s.Options)
			},
		},
		{
			name: "body that is not an assertion", ttl: passkeytest.CeremonyTimeout,
			prepare: func(t *testing.T, _ *ceremony.Ceremony, _ *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				return s.Token, []byte(`{"id":"nope"}`)
			},
		},
		{
			name: "assertion for another ceremony's challenge", ttl: passkeytest.CeremonyTimeout,
			prepare: func(t *testing.T, c *ceremony.Ceremony, acc *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				other, err := c.BeginLogin()
				require.NoError(t, err)

				return other.Token, acc.assert(t, s.Options)
			},
		},
	}
}

func loginAccountCases() []loginCase {
	return []loginCase{
		{
			name: "assertion without user verification", ttl: passkeytest.CeremonyTimeout,
			prepare: func(t *testing.T, _ *ceremony.Ceremony, acc *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				acc.auth.Options.UserNotVerified = true

				return s.Token, acc.assert(t, s.Options)
			},
		},
		{
			name: "no user handle", ttl: passkeytest.CeremonyTimeout,
			prepare: func(t *testing.T, _ *ceremony.Ceremony, acc *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				acc.auth.Options.UserHandle = nil

				return s.Token, acc.assert(t, s.Options)
			},
		},
		{
			name: "user handle that is not a UUID", ttl: passkeytest.CeremonyTimeout,
			prepare: func(t *testing.T, _ *ceremony.Ceremony, acc *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				acc.auth.Options.UserHandle = []byte("short")

				return s.Token, acc.assert(t, s.Options)
			},
		},
		{
			name: "unknown user handle", ttl: passkeytest.CeremonyTimeout,
			prepare: func(t *testing.T, _ *ceremony.Ceremony, acc *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				other := uuid.New()
				acc.auth.Options.UserHandle = other[:]

				return s.Token, acc.assert(t, s.Options)
			},
		},
	}
}

// loginCredentialCases name a credential that cannot sign in.
func loginCredentialCases() []loginCase {
	return []loginCase{
		{
			name: "unknown credential", ttl: passkeytest.CeremonyTimeout,
			prepare: func(t *testing.T, _ *ceremony.Ceremony, acc *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				acc.passkey.ID = entity.PasskeyID{9, 9, 9}

				return s.Token, acc.assert(t, s.Options)
			},
		},
		{
			name: "credential signed by another key", ttl: passkeytest.CeremonyTimeout,
			prepare: func(t *testing.T, _ *ceremony.Ceremony, acc *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				forged := newCred()
				forged.ID = acc.cred.ID
				acc.cred = forged

				return s.Token, acc.assert(t, s.Options)
			},
		},
		{
			name: "sign count regression", ttl: passkeytest.CeremonyTimeout,
			prepare: func(t *testing.T, _ *ceremony.Ceremony, acc *account, s ceremony.Started) (string, []byte) {
				t.Helper()

				acc.passkey.Credential = passkeytest.WithSignCount(t, acc.passkey.Credential, 5)
				acc.cred.Counter = 3

				return s.Token, acc.assert(t, s.Options)
			},
		},
	}
}

func TestFinishLoginRefusesAnInvalidCeremony(t *testing.T) {
	t.Parallel()

	for _, tc := range slices.Concat(loginTokenCases(), loginAccountCases(), loginCredentialCases()) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Register with a working ceremony; the stored passkey is plain data any ceremony can use.
			acc := register(t, newCeremony(t, passkeytest.CeremonyTimeout, nil))
			c := newCeremony(t, tc.ttl, nil)

			started, err := c.BeginLogin()
			require.NoError(t, err)

			token, body := tc.prepare(t, c, acc, started)

			_, err = c.FinishLogin(token, body, acc.lookup)
			require.ErrorIs(t, err, entity.ErrPasskeyCeremonyInvalid)
		})
	}
}
