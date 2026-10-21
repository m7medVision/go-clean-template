package usecase_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/descope/virtualwebauthn"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/usecase"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	_rpID            = "localhost"
	_rpName          = "Go Clean Template"
	_rpOrigin        = "http://localhost:8080"
	_ceremonyPurpose = "webauthn-ceremony"
	_ceremonyTimeout = 5 * time.Minute
	_reauthMaxAge    = 5 * time.Minute
)

type passkeyFixture struct {
	uc       usecase.Passkey
	users    *MockUserRepo
	passkeys *MockPasskeyRepo
	tokens   *jwt.Manager
}

func newPasskeyUseCase(t *testing.T, ttl time.Duration) passkeyFixture {
	t.Helper()

	return newPasskeyUseCaseAt(t, ttl, nil)
}

// newPasskeyUseCaseAt is newPasskeyUseCase with the use case's clock set to now (nil: the real one).
func newPasskeyUseCaseAt(t *testing.T, ttl time.Duration, now func() time.Time) passkeyFixture {
	t.Helper()

	ctrl := gomock.NewController(t)

	users := NewMockUserRepo(ctrl)
	passkeys := NewMockPasskeyRepo(ctrl)

	ceremony, err := jwt.NewScoped("test-secret", _ceremonyPurpose, ttl)
	require.NoError(t, err)

	tokens := jwt.New("test-secret", time.Hour)

	uc, err := passkey.New(users, passkeys, ceremony, tokens, &passkey.Config{
		RPID:         _rpID,
		RPName:       _rpName,
		RPOrigins:    []string{_rpOrigin},
		Timeout:      _ceremonyTimeout,
		ReauthMaxAge: _reauthMaxAge,
		Now:          now,
	})
	require.NoError(t, err)

	return passkeyFixture{uc: uc, users: users, passkeys: passkeys, tokens: tokens}
}

func passkeyUser() entity.User {
	return entity.User{ID: uuid.NewString(), Username: "alice", Email: "alice@example.com"}
}

// expectBegin sets up the reads BeginRegistration makes for user, with stored as their passkeys.
func (f passkeyFixture) expectBegin(user *entity.User, stored ...entity.Passkey) {
	f.users.EXPECT().GetByID(gomock.Any(), user.ID).Return(*user, nil)
	f.passkeys.EXPECT().ListByUser(gomock.Any(), user.ID).Return(stored, nil)
}

// attest plays the browser and authenticator: it answers the creation options with an attestation
// for cred, as navigator.credentials.create and PublicKeyCredential.toJSON would.
func attest(t *testing.T, options []byte, cred virtualwebauthn.Credential) []byte {
	t.Helper()

	auth := virtualwebauthn.NewAuthenticator()

	return attestWith(t, options, cred, &auth)
}

// attestWith is attest from a given authenticator.
func attestWith(t *testing.T, options []byte, cred virtualwebauthn.Credential, auth *virtualwebauthn.Authenticator) []byte {
	t.Helper()

	parsed, err := virtualwebauthn.ParseAttestationOptions(string(options))
	require.NoError(t, err)

	rp := virtualwebauthn.RelyingParty{ID: _rpID, Name: _rpName, Origin: _rpOrigin}

	return []byte(virtualwebauthn.CreateAttestationResponse(rp, *auth, cred, *parsed))
}

func TestPasskeyRegistration(t *testing.T) {
	t.Parallel()

	t.Run("begin, attest, finish stores the passkey", func(t *testing.T) {
		t.Parallel()

		f := newPasskeyUseCase(t, _ceremonyTimeout)
		user := passkeyUser()
		f.expectBegin(&user)

		options, token, err := f.uc.BeginRegistration(context.Background(), user.ID, time.Now())
		require.NoError(t, err)
		require.NotEmpty(t, token)

		cred := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)

		var stored entity.Passkey

		f.passkeys.EXPECT().Store(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, p *entity.Passkey) error {
				stored = *p

				return nil
			})

		got, err := f.uc.FinishRegistration(context.Background(), user.ID, token, "Laptop", attest(t, options, cred))
		require.NoError(t, err)

		assert.Equal(t, entity.PasskeyID(cred.ID), got.ID)
		assert.Equal(t, user.ID, got.UserID)
		assert.Equal(t, "Laptop", got.Name)
		assert.False(t, got.CreatedAt.IsZero())
		assert.Nil(t, got.LastUsedAt)
		assert.Equal(t, got, stored)

		var record webauthn.Credential

		require.NoError(t, json.Unmarshal(got.Credential, &record))
		assert.Equal(t, cred.ID, record.ID)
		assert.NotEmpty(t, record.PublicKey)
	})
}

func TestPasskeyFinishRegistration_DefaultName(t *testing.T) {
	t.Parallel()

	t.Run("an empty name gets the default", func(t *testing.T) {
		t.Parallel()

		f := newPasskeyUseCase(t, _ceremonyTimeout)
		user := passkeyUser()
		f.expectBegin(&user)

		options, token, err := f.uc.BeginRegistration(context.Background(), user.ID, time.Now())
		require.NoError(t, err)

		f.passkeys.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)

		body := attest(t, options, virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2))

		got, err := f.uc.FinishRegistration(context.Background(), user.ID, token, "  ", body)
		require.NoError(t, err)
		assert.Equal(t, "Passkey", got.Name)
	})
}

func TestPasskeyFinishRegistration_StoreErrors(t *testing.T) {
	t.Parallel()

	t.Run("an already registered credential is ErrPasskeyExists", func(t *testing.T) {
		t.Parallel()

		f := newPasskeyUseCase(t, _ceremonyTimeout)
		user := passkeyUser()
		f.expectBegin(&user)

		options, token, err := f.uc.BeginRegistration(context.Background(), user.ID, time.Now())
		require.NoError(t, err)

		f.passkeys.EXPECT().Store(gomock.Any(), gomock.Any()).Return(entity.ErrPasskeyExists)

		body := attest(t, options, virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2))

		_, err = f.uc.FinishRegistration(context.Background(), user.ID, token, "Laptop", body)
		require.ErrorIs(t, err, entity.ErrPasskeyExists)
	})

	t.Run("a store failure is wrapped", func(t *testing.T) {
		t.Parallel()

		f := newPasskeyUseCase(t, _ceremonyTimeout)
		user := passkeyUser()
		f.expectBegin(&user)

		options, token, err := f.uc.BeginRegistration(context.Background(), user.ID, time.Now())
		require.NoError(t, err)

		f.passkeys.EXPECT().Store(gomock.Any(), gomock.Any()).Return(errInternalServErr)

		body := attest(t, options, virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2))

		_, err = f.uc.FinishRegistration(context.Background(), user.ID, token, "Laptop", body)
		require.ErrorIs(t, err, errInternalServErr)
		require.NotErrorIs(t, err, entity.ErrPasskeyCeremonyInvalid)
	})
}

func TestPasskeyBeginRegistration_Options(t *testing.T) {
	t.Parallel()

	f := newPasskeyUseCase(t, _ceremonyTimeout)
	user := passkeyUser()
	existing, err := json.Marshal(webauthn.Credential{ID: []byte{1, 2, 3, 4}})
	require.NoError(t, err)

	f.expectBegin(&user, entity.Passkey{ID: entity.PasskeyID{1, 2, 3, 4}, UserID: user.ID, Credential: existing})

	options, _, err := f.uc.BeginRegistration(context.Background(), user.ID, time.Now())
	require.NoError(t, err)

	parsed, err := virtualwebauthn.ParseAttestationOptions(string(options))
	require.NoError(t, err)
	assert.Equal(t, _rpID, parsed.RelyingPartyID)
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

	require.NoError(t, json.Unmarshal(options, &raw))
	assert.Equal(t, "none", raw.Attestation)
	assert.Equal(t, _ceremonyTimeout.Milliseconds(), raw.Timeout)
	assert.Equal(t, "required", raw.AuthenticatorSelection.ResidentKey)
	assert.True(t, raw.AuthenticatorSelection.RequireResident)
	assert.Equal(t, "required", raw.AuthenticatorSelection.UserVerification)
}

func TestPasskeyBeginRegistration_Reauth(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	for _, tt := range []struct {
		name     string
		authTime time.Time
		wantErr  error
	}{
		{name: "signed in just now", authTime: now},
		{name: "signed in exactly the max age ago", authTime: now.Add(-_reauthMaxAge)},
		{name: "signed in longer ago", authTime: now.Add(-_reauthMaxAge - time.Second), wantErr: entity.ErrPasskeyReauthRequired},
		{name: "token without an issue time", authTime: time.Time{}, wantErr: entity.ErrPasskeyReauthRequired},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newPasskeyUseCaseAt(t, _ceremonyTimeout, func() time.Time { return now })
			user := passkeyUser()

			// A stale sign-in is refused before anything is read: no repo expectation then.
			if tt.wantErr == nil {
				f.expectBegin(&user)
			}

			options, token, err := f.uc.BeginRegistration(context.Background(), user.ID, tt.authTime)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, options)
				assert.Empty(t, token)

				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, options)
			assert.NotEmpty(t, token)
		})
	}
}

type ceremonyCase struct {
	name string
	ttl  time.Duration
	// finish returns the arguments FinishRegistration is called with, from a begun ceremony.
	finish func(t *testing.T, f passkeyFixture, user entity.User, options []byte, token string) (string, string, []byte)
}

// ceremonyInvalidCases are finishes that must fail as ErrPasskeyCeremonyInvalid.
func ceremonyInvalidCases() []ceremonyCase {
	return append(tokenInvalidCases(), attestationInvalidCases()...)
}

// tokenInvalidCases break the ceremony token, or finish it for the wrong user.
func tokenInvalidCases() []ceremonyCase {
	return []ceremonyCase{
		{
			name: "tampered token",
			ttl:  _ceremonyTimeout,
			finish: func(t *testing.T, _ passkeyFixture, user entity.User, options []byte, token string) (string, string, []byte) {
				t.Helper()

				return user.ID, tamper(t, token), attest(t, options, virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2))
			},
		},
		{
			name: "expired token",
			ttl:  -time.Minute,
			finish: func(t *testing.T, _ passkeyFixture, user entity.User, options []byte, token string) (string, string, []byte) {
				t.Helper()

				return user.ID, token, attest(t, options, virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2))
			},
		},
		{
			name: "token for another user",
			ttl:  _ceremonyTimeout,
			finish: func(t *testing.T, _ passkeyFixture, _ entity.User, options []byte, token string) (string, string, []byte) {
				t.Helper()

				return uuid.NewString(), token, attest(t, options, virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2))
			},
		},
	}
}

// attestationInvalidCases are bodies that do not verify against the ceremony, or a caller it cannot
// belong to.
func attestationInvalidCases() []ceremonyCase {
	return []ceremonyCase{
		{
			name: "attestation for another ceremony's challenge",
			ttl:  _ceremonyTimeout,
			finish: func(t *testing.T, f passkeyFixture, user entity.User, options []byte, _ string) (string, string, []byte) {
				t.Helper()

				f.expectBegin(&user)

				_, other, err := f.uc.BeginRegistration(context.Background(), user.ID, time.Now())
				require.NoError(t, err)

				return user.ID, other, attest(t, options, virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2))
			},
		},
		{
			name: "body that is not a credential",
			ttl:  _ceremonyTimeout,
			finish: func(t *testing.T, _ passkeyFixture, user entity.User, _ []byte, token string) (string, string, []byte) {
				t.Helper()

				return user.ID, token, []byte(`{"id":"nope"}`)
			},
		},
		{
			name: "attestation without user verification",
			ttl:  _ceremonyTimeout,
			finish: func(t *testing.T, _ passkeyFixture, user entity.User, options []byte, token string) (string, string, []byte) {
				t.Helper()

				unverified := virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{UserNotVerified: true})

				return user.ID, token, attestWith(t, options, virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2), &unverified)
			},
		},
		{
			name: "user ID that is not a UUID",
			ttl:  _ceremonyTimeout,
			finish: func(t *testing.T, _ passkeyFixture, _ entity.User, options []byte, token string) (string, string, []byte) {
				t.Helper()

				return "not-a-uuid", token, attest(t, options, virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2))
			},
		},
	}
}

func TestPasskeyFinishRegistration_CeremonyInvalid(t *testing.T) {
	t.Parallel()

	for _, tc := range ceremonyInvalidCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newPasskeyUseCase(t, tc.ttl)
			user := passkeyUser()
			f.expectBegin(&user)

			options, token, err := f.uc.BeginRegistration(context.Background(), user.ID, time.Now())
			require.NoError(t, err)

			userID, token, body := tc.finish(t, f, user, options, token)

			// No Store expectation: gomock fails the test if an invalid ceremony reaches the repo.
			_, err = f.uc.FinishRegistration(context.Background(), userID, token, "Laptop", body)
			require.ErrorIs(t, err, entity.ErrPasskeyCeremonyInvalid)
		})
	}
}

// tamper flips one byte of the token's payload segment, keeping it valid base64url.
func tamper(t *testing.T, token string) string {
	t.Helper()

	b := []byte(token)
	i := len(b) / 2

	require.NotEqual(t, byte('.'), b[i])

	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}

	return string(b)
}

func TestPasskeyBeginRegistration_RepoErrors(t *testing.T) {
	t.Parallel()

	t.Run("user lookup failure is wrapped", func(t *testing.T) {
		t.Parallel()

		f := newPasskeyUseCase(t, _ceremonyTimeout)
		f.users.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(entity.User{}, entity.ErrUserNotFound)

		_, _, err := f.uc.BeginRegistration(context.Background(), uuid.NewString(), time.Now())
		require.ErrorIs(t, err, entity.ErrUserNotFound)
	})

	t.Run("passkey list failure is wrapped", func(t *testing.T) {
		t.Parallel()

		f := newPasskeyUseCase(t, _ceremonyTimeout)
		user := passkeyUser()
		f.users.EXPECT().GetByID(gomock.Any(), user.ID).Return(user, nil)
		f.passkeys.EXPECT().ListByUser(gomock.Any(), user.ID).Return(nil, errInternalServErr)

		_, _, err := f.uc.BeginRegistration(context.Background(), user.ID, time.Now())
		require.ErrorIs(t, err, errInternalServErr)
	})

	t.Run("a stored credential that does not decode fails", func(t *testing.T) {
		t.Parallel()

		f := newPasskeyUseCase(t, _ceremonyTimeout)
		user := passkeyUser()
		f.expectBegin(&user, entity.Passkey{ID: entity.PasskeyID{1}, Credential: json.RawMessage(`"x"`)})

		_, _, err := f.uc.BeginRegistration(context.Background(), user.ID, time.Now())
		require.Error(t, err)
	})
}

func TestPasskeyList(t *testing.T) {
	t.Parallel()

	t.Run("returns the user's passkeys", func(t *testing.T) {
		t.Parallel()

		f := newPasskeyUseCase(t, _ceremonyTimeout)
		want := []entity.Passkey{{ID: entity.PasskeyID("k"), Name: "Laptop"}}
		f.passkeys.EXPECT().ListByUser(gomock.Any(), "u1").Return(want, nil)

		got, err := f.uc.List(context.Background(), "u1")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("repo failure is wrapped", func(t *testing.T) {
		t.Parallel()

		f := newPasskeyUseCase(t, _ceremonyTimeout)
		f.passkeys.EXPECT().ListByUser(gomock.Any(), "u1").Return(nil, errInternalServErr)

		_, err := f.uc.List(context.Background(), "u1")
		require.ErrorIs(t, err, errInternalServErr)
	})
}

// _passkeyIDText is a passkey ID as the API carries it: unpadded base64url of 1, 2, 3, 4.
const _passkeyIDText = "AQIDBA"

type renameCase struct {
	name      string
	id        string
	newName   string
	storeAs   string // the name the repo is asked to store; empty means the repo is not called
	repoErr   error
	wantErr   error
	wantLabel string
}

func renameCases() []renameCase {
	return []renameCase{
		{name: "renames the user's passkey", id: _passkeyIDText, newName: "Work laptop", storeAs: "Work laptop", wantLabel: "Work laptop"},
		{name: "trims the name", id: _passkeyIDText, newName: "  Phone \t", storeAs: "Phone", wantLabel: "Phone"},
		{name: "a blank name gets the default", id: _passkeyIDText, newName: "   ", storeAs: "Passkey", wantLabel: "Passkey"},
		{name: "an id that is not base64url is not found", id: "not base64!", newName: "Phone", wantErr: entity.ErrPasskeyNotFound},
		{
			name: "another user's or a missing passkey is not found", id: _passkeyIDText, newName: "Phone", storeAs: "Phone",
			repoErr: entity.ErrPasskeyNotFound, wantErr: entity.ErrPasskeyNotFound,
		},
		{
			name: "repo failure is wrapped", id: _passkeyIDText, newName: "Phone", storeAs: "Phone",
			repoErr: errInternalServErr, wantErr: errInternalServErr,
		},
	}
}

func TestPasskeyRename(t *testing.T) {
	t.Parallel()

	for _, tc := range renameCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newPasskeyUseCase(t, _ceremonyTimeout)

			if tc.storeAs != "" {
				f.passkeys.EXPECT().Rename(gomock.Any(), "u1", entity.PasskeyID{1, 2, 3, 4}, tc.storeAs).
					DoAndReturn(func(_ context.Context, userID string, id entity.PasskeyID, name string) (entity.Passkey, error) {
						if tc.repoErr != nil {
							return entity.Passkey{}, tc.repoErr
						}

						return entity.Passkey{ID: id, UserID: userID, Name: name}, nil
					})
			}

			got, err := f.uc.Rename(context.Background(), "u1", tc.id, tc.newName)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, entity.PasskeyID{1, 2, 3, 4}, got.ID)
			assert.Equal(t, tc.wantLabel, got.Name)
		})
	}
}

func TestPasskeyDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       string
		callRepo bool
		repoErr  error
		wantErr  error
	}{
		{name: "deletes the user's passkey", id: _passkeyIDText, callRepo: true},
		{name: "an id that is not base64url is not found", id: "not base64!", wantErr: entity.ErrPasskeyNotFound},
		{
			name: "another user's or a missing passkey is not found", id: _passkeyIDText, callRepo: true,
			repoErr: entity.ErrPasskeyNotFound, wantErr: entity.ErrPasskeyNotFound,
		},
		{name: "repo failure is wrapped", id: _passkeyIDText, callRepo: true, repoErr: errInternalServErr, wantErr: errInternalServErr},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newPasskeyUseCase(t, _ceremonyTimeout)

			if tc.callRepo {
				f.passkeys.EXPECT().Delete(gomock.Any(), "u1", entity.PasskeyID{1, 2, 3, 4}).Return(tc.repoErr)
			}

			err := f.uc.Delete(context.Background(), "u1", tc.id)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}

// loginAccount is a user with a passkey registered through the real registration ceremony, and the
// virtual authenticator holding it.
type loginAccount struct {
	user    entity.User
	passkey entity.Passkey
	cred    virtualwebauthn.Credential
	auth    virtualwebauthn.Authenticator
}

// registerPasskey registers a fresh credential for a new user and returns what was stored.
func registerPasskey(t *testing.T, f passkeyFixture) loginAccount {
	t.Helper()

	user := passkeyUser()
	f.expectBegin(&user)

	options, token, err := f.uc.BeginRegistration(context.Background(), user.ID, time.Now())
	require.NoError(t, err)

	cred := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)

	f.passkeys.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)

	stored, err := f.uc.FinishRegistration(context.Background(), user.ID, token, "Laptop", attest(t, options, cred))
	require.NoError(t, err)

	handle := uuid.MustParse(user.ID)

	return loginAccount{
		user:    user,
		passkey: stored,
		cred:    cred,
		auth:    virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{UserHandle: handle[:]}),
	}
}

// assertLogin plays the browser for a discoverable login: it answers the request options with an
// assertion from acc's authenticator, as navigator.credentials.get and toJSON would.
func assertLogin(t *testing.T, options []byte, acc *loginAccount) []byte {
	t.Helper()

	parsed, err := virtualwebauthn.ParseAssertionOptions(string(options))
	require.NoError(t, err)
	assert.Empty(t, parsed.AllowCredentials, "a discoverable login names no credentials")

	rp := virtualwebauthn.RelyingParty{ID: _rpID, Name: _rpName, Origin: _rpOrigin}

	return []byte(virtualwebauthn.CreateAssertionResponse(rp, acc.auth, acc.cred, *parsed))
}

// expectLookup sets up the reads the discoverable handler makes for acc.
func (f passkeyFixture) expectLookup(acc *loginAccount) {
	f.users.EXPECT().GetByID(gomock.Any(), acc.user.ID).Return(acc.user, nil)
	f.passkeys.EXPECT().ListByUser(gomock.Any(), acc.user.ID).Return([]entity.Passkey{acc.passkey}, nil)
}

// burnStore is an in-memory webauthn_used_challenges: BurnChallenge succeeds once per hash.
type burnStore struct {
	mu   sync.Mutex
	used map[string]time.Time
}

func (b *burnStore) burn(_ context.Context, hash []byte, expiresAt time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.used[string(hash)]; ok {
		return entity.ErrPasskeyChallengeUsed
	}

	b.used[string(hash)] = expiresAt

	return nil
}

// loginChallenge checks the request options BeginLogin returns and returns their challenge.
func loginChallenge(t *testing.T, options []byte) string {
	t.Helper()

	var raw struct {
		Challenge        string `json:"challenge"`
		RPID             string `json:"rpId"`
		Timeout          int64  `json:"timeout"`
		UserVerification string `json:"userVerification"`
	}

	require.NoError(t, json.Unmarshal(options, &raw))
	assert.Equal(t, _rpID, raw.RPID)
	assert.Equal(t, _ceremonyTimeout.Milliseconds(), raw.Timeout)
	assert.Equal(t, "required", raw.UserVerification)
	require.NotEmpty(t, raw.Challenge)

	return raw.Challenge
}

func TestPasskeyLogin(t *testing.T) {
	t.Parallel()

	f := newPasskeyUseCase(t, _ceremonyTimeout)
	acc := registerPasskey(t, f)
	acc.cred.Counter = 1

	options, token, err := f.uc.BeginLogin(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, token)

	challenge := sha256.Sum256([]byte(loginChallenge(t, options)))

	burns := &burnStore{used: map[string]time.Time{}}

	var (
		updated entity.Passkey
		usedAt  time.Time
	)

	f.expectLookup(&acc)
	gomock.InOrder(
		f.passkeys.EXPECT().DeleteExpiredChallenges(gomock.Any()).Return(nil),
		f.passkeys.EXPECT().BurnChallenge(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(burns.burn),
		f.passkeys.EXPECT().UpdateUsage(gomock.Any(), acc.passkey.ID, gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ entity.PasskeyID, record json.RawMessage, at time.Time) error {
				updated.Credential, usedAt = record, at

				return nil
			}),
	)

	body := assertLogin(t, options, &acc)
	before := time.Now()

	authToken, err := f.uc.FinishLogin(context.Background(), token, body)
	require.NoError(t, err)

	subject, err := f.tokens.ParseToken(authToken)
	require.NoError(t, err)
	assert.Equal(t, acc.user.ID, subject)

	require.Contains(t, burns.used, string(challenge[:]), "the burned hash is the challenge's SHA-256")
	assert.WithinDuration(t, before.Add(_ceremonyTimeout), burns.used[string(challenge[:])], time.Minute)

	var record webauthn.Credential

	require.NoError(t, json.Unmarshal(updated.Credential, &record))
	assert.Equal(t, uint32(1), record.Authenticator.SignCount)
	assert.False(t, record.Authenticator.CloneWarning)
	assert.WithinDuration(t, time.Now(), usedAt, time.Minute)

	t.Run("finishing the same ceremony again is a replay", func(t *testing.T) {
		t.Parallel()

		f.expectLookup(&acc)
		f.passkeys.EXPECT().DeleteExpiredChallenges(gomock.Any()).Return(nil)
		f.passkeys.EXPECT().BurnChallenge(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(burns.burn)

		// No UpdateUsage expectation: a replay must not touch the credential or issue a token.
		replayed, err := f.uc.FinishLogin(context.Background(), token, body)
		require.ErrorIs(t, err, entity.ErrPasskeyChallengeUsed)
		assert.Empty(t, replayed)
	})
}

type loginCase struct {
	name string
	ttl  time.Duration
	// prepare sets up the repo calls the case expects and returns the FinishLogin arguments.
	prepare func(t *testing.T, f passkeyFixture, acc *loginAccount, options []byte, token string) (string, []byte)
}

func loginRejectedCases() []loginCase {
	return append(append(tokenRejectedCases(), assertionRejectedCases()...), accountRejectedCases()...)
}

// tokenRejectedCases break the ceremony token or the body's shape.
func tokenRejectedCases() []loginCase {
	return []loginCase{
		{
			name: "tampered token",
			ttl:  _ceremonyTimeout,
			prepare: func(t *testing.T, _ passkeyFixture, acc *loginAccount, options []byte, token string) (string, []byte) {
				t.Helper()

				return tamper(t, token), assertLogin(t, options, acc)
			},
		},
		{
			name: "expired token",
			ttl:  -time.Minute,
			prepare: func(t *testing.T, _ passkeyFixture, acc *loginAccount, options []byte, token string) (string, []byte) {
				t.Helper()

				return token, assertLogin(t, options, acc)
			},
		},
		{
			name: "registration token",
			ttl:  _ceremonyTimeout,
			prepare: func(t *testing.T, f passkeyFixture, acc *loginAccount, options []byte, _ string) (string, []byte) {
				t.Helper()

				f.expectBegin(&acc.user)

				_, other, err := f.uc.BeginRegistration(context.Background(), acc.user.ID, time.Now())
				require.NoError(t, err)

				return other, assertLogin(t, options, acc)
			},
		},
		{
			name: "body that is not an assertion",
			ttl:  _ceremonyTimeout,
			prepare: func(t *testing.T, _ passkeyFixture, _ *loginAccount, _ []byte, token string) (string, []byte) {
				t.Helper()

				return token, []byte(`{"id":"nope"}`)
			},
		},
	}
}

// assertionRejectedCases are well-formed assertions that do not verify.
func assertionRejectedCases() []loginCase {
	return []loginCase{
		{
			name: "assertion for another ceremony's challenge",
			ttl:  _ceremonyTimeout,
			prepare: func(t *testing.T, f passkeyFixture, acc *loginAccount, options []byte, _ string) (string, []byte) {
				t.Helper()

				_, other, err := f.uc.BeginLogin(context.Background())
				require.NoError(t, err)

				f.expectLookup(acc)

				return other, assertLogin(t, options, acc)
			},
		},
		{
			name: "assertion without user verification",
			ttl:  _ceremonyTimeout,
			prepare: func(t *testing.T, f passkeyFixture, acc *loginAccount, options []byte, token string) (string, []byte) {
				t.Helper()

				f.expectLookup(acc)

				acc.auth.Options.UserNotVerified = true

				return token, assertLogin(t, options, acc)
			},
		},
		{
			name: "no user handle",
			ttl:  _ceremonyTimeout,
			prepare: func(t *testing.T, _ passkeyFixture, acc *loginAccount, options []byte, token string) (string, []byte) {
				t.Helper()

				acc.auth.Options.UserHandle = nil

				return token, assertLogin(t, options, acc)
			},
		},
	}
}

// accountRejectedCases name an account or credential that cannot sign in.
func accountRejectedCases() []loginCase {
	return []loginCase{
		{
			name: "user handle that is not a UUID",
			ttl:  _ceremonyTimeout,
			prepare: func(t *testing.T, _ passkeyFixture, acc *loginAccount, options []byte, token string) (string, []byte) {
				t.Helper()

				acc.auth.Options.UserHandle = []byte("short")

				return token, assertLogin(t, options, acc)
			},
		},
		{
			name: "unknown user handle",
			ttl:  _ceremonyTimeout,
			prepare: func(t *testing.T, f passkeyFixture, acc *loginAccount, options []byte, token string) (string, []byte) {
				t.Helper()

				f.users.EXPECT().GetByID(gomock.Any(), acc.user.ID).Return(entity.User{}, entity.ErrUserNotFound)

				return token, assertLogin(t, options, acc)
			},
		},
		{
			name: "unknown credential",
			ttl:  _ceremonyTimeout,
			prepare: func(t *testing.T, f passkeyFixture, acc *loginAccount, options []byte, token string) (string, []byte) {
				t.Helper()

				f.users.EXPECT().GetByID(gomock.Any(), acc.user.ID).Return(acc.user, nil)
				f.passkeys.EXPECT().ListByUser(gomock.Any(), acc.user.ID).Return(nil, nil)

				return token, assertLogin(t, options, acc)
			},
		},
		{
			name: "credential signed by another key",
			ttl:  _ceremonyTimeout,
			prepare: func(t *testing.T, f passkeyFixture, acc *loginAccount, options []byte, token string) (string, []byte) {
				t.Helper()

				f.expectLookup(acc)

				forged := virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2)
				forged.ID = acc.cred.ID
				acc.cred = forged

				return token, assertLogin(t, options, acc)
			},
		},
		{
			name: "sign count regression",
			ttl:  _ceremonyTimeout,
			prepare: func(t *testing.T, f passkeyFixture, acc *loginAccount, options []byte, token string) (string, []byte) {
				t.Helper()

				acc.passkey.Credential = withSignCount(t, acc.passkey.Credential, 5)
				acc.cred.Counter = 3
				f.expectLookup(acc)

				return token, assertLogin(t, options, acc)
			},
		},
	}
}

// withSignCount returns the stored credential record with its sign count set to n.
func withSignCount(t *testing.T, record json.RawMessage, n uint32) json.RawMessage {
	t.Helper()

	var credential webauthn.Credential

	require.NoError(t, json.Unmarshal(record, &credential))

	credential.Authenticator.SignCount = n

	out, err := json.Marshal(credential)
	require.NoError(t, err)

	return out
}

func TestPasskeyFinishLogin_CeremonyInvalid(t *testing.T) {
	t.Parallel()

	for _, tc := range loginRejectedCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Register with a working ceremony; the stored passkey is plain data any fixture can use.
			acc := registerPasskey(t, newPasskeyUseCase(t, _ceremonyTimeout))
			f := newPasskeyUseCase(t, tc.ttl)

			options, token, err := f.uc.BeginLogin(context.Background())
			require.NoError(t, err)

			token, body := tc.prepare(t, f, &acc, options, token)

			// No burn or update expectation: gomock fails the test if a rejected login reaches them.
			got, err := f.uc.FinishLogin(context.Background(), token, body)
			require.ErrorIs(t, err, entity.ErrPasskeyCeremonyInvalid)
			assert.Empty(t, got)
		})
	}
}

func TestPasskeyFinishLogin_SignCountZero(t *testing.T) {
	t.Parallel()

	t.Run("authenticators without a counter keep signing in", func(t *testing.T) {
		t.Parallel()

		f := newPasskeyUseCase(t, _ceremonyTimeout)
		acc := registerPasskey(t, f)

		options, token, err := f.uc.BeginLogin(context.Background())
		require.NoError(t, err)

		f.expectLookup(&acc)
		f.passkeys.EXPECT().DeleteExpiredChallenges(gomock.Any()).Return(nil)
		f.passkeys.EXPECT().BurnChallenge(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		f.passkeys.EXPECT().UpdateUsage(gomock.Any(), acc.passkey.ID, gomock.Any(), gomock.Any()).Return(nil)

		got, err := f.uc.FinishLogin(context.Background(), token, assertLogin(t, options, &acc))
		require.NoError(t, err)
		assert.NotEmpty(t, got)
	})
}

type loginRepoCase struct {
	name   string
	expect func(f passkeyFixture, acc *loginAccount)
}

func loginRepoErrorCases() []loginRepoCase {
	return []loginRepoCase{
		{
			name: "user lookup failure",
			expect: func(f passkeyFixture, acc *loginAccount) {
				f.users.EXPECT().GetByID(gomock.Any(), acc.user.ID).Return(entity.User{}, errInternalServErr)
			},
		},
		{
			name: "passkey list failure",
			expect: func(f passkeyFixture, acc *loginAccount) {
				f.users.EXPECT().GetByID(gomock.Any(), acc.user.ID).Return(acc.user, nil)
				f.passkeys.EXPECT().ListByUser(gomock.Any(), acc.user.ID).Return(nil, errInternalServErr)
			},
		},
		{
			name: "expired challenge cleanup failure",
			expect: func(f passkeyFixture, acc *loginAccount) {
				f.expectLookup(acc)
				f.passkeys.EXPECT().DeleteExpiredChallenges(gomock.Any()).Return(errInternalServErr)
			},
		},
		{
			name: "burn failure",
			expect: func(f passkeyFixture, acc *loginAccount) {
				f.expectLookup(acc)
				f.passkeys.EXPECT().DeleteExpiredChallenges(gomock.Any()).Return(nil)
				f.passkeys.EXPECT().BurnChallenge(gomock.Any(), gomock.Any(), gomock.Any()).Return(errInternalServErr)
			},
		},
		{
			name: "usage update failure",
			expect: func(f passkeyFixture, acc *loginAccount) {
				f.expectLookup(acc)
				f.passkeys.EXPECT().DeleteExpiredChallenges(gomock.Any()).Return(nil)
				f.passkeys.EXPECT().BurnChallenge(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				f.passkeys.EXPECT().UpdateUsage(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errInternalServErr)
			},
		},
	}
}

func TestPasskeyFinishLogin_RepoErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range loginRepoErrorCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newPasskeyUseCase(t, _ceremonyTimeout)
			acc := registerPasskey(t, f)

			options, token, err := f.uc.BeginLogin(context.Background())
			require.NoError(t, err)

			tc.expect(f, &acc)

			got, err := f.uc.FinishLogin(context.Background(), token, assertLogin(t, options, &acc))
			require.ErrorIs(t, err, errInternalServErr)
			require.NotErrorIs(t, err, entity.ErrPasskeyCeremonyInvalid)
			assert.Empty(t, got)
		})
	}
}
