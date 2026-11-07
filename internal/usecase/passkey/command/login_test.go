package command_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/ceremony/ceremonytest"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/repo/session"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/command"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// login is a login handler over the fixture's store and ceremony, with users finding the fixture's
// user and guard keeping the burned challenges.
type login struct {
	handler *command.FinishPasskeyLoginHandler
	guard   *burnStore
	tokens  *jwt.Manager
}

func (f *fixture) login(t *testing.T) login {
	t.Helper()

	users := repomock.NewMockUserReadModel(gomock.NewController(t))
	users.EXPECT().ByID(gomock.Any(), f.user.ID).Return(f.user, nil).AnyTimes()

	guard := newBurnStore()
	tokens := jwt.New(ceremonytest.SigningKey, time.Hour)

	return login{
		handler: &command.FinishPasskeyLoginHandler{
			Ceremony: f.ceremony, Store: f.store, Users: users, Guard: guard, Sessions: session.New(tokens), Now: now,
		},
		guard:  guard,
		tokens: tokens,
	}
}

// assertion begins a login and answers it with the fixture's credential.
func (f *fixture) assertion(t *testing.T) (token string, body []byte, challenge string) {
	t.Helper()

	started, err := f.ceremony.BeginLogin()
	require.NoError(t, err)

	var raw struct {
		Challenge string `json:"challenge"`
	}

	require.NoError(t, json.Unmarshal(started.Options, &raw))

	return started.Token, ceremonytest.Assert(t, started.Options, f.cred, &f.auth), raw.Challenge
}

func TestFinishPasskeyLoginSignsInAndRecordsTheUse(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.register(t)
	f.cred.Counter = 1
	l := f.login(t)

	token, body, challenge := f.assertion(t)

	signedIn, err := l.handler.Handle(context.Background(), command.FinishPasskeyLogin{Token: token, Body: body})
	require.NoError(t, err)

	claims, err := l.tokens.Parse(signedIn.Token)
	require.NoError(t, err)
	assert.Equal(t, f.user.ID, claims.UserID)
	assert.Equal(t, []string{"user"}, claims.Roles, "a passkey sign-in grants only the user role")
	assert.Equal(t, id.String(), signedIn.ID)
	assert.Positive(t, signedIn.Position)

	hash := sha256.Sum256([]byte(challenge))
	assert.Contains(t, l.guard.used, string(hash[:]))

	used, ok := f.events(id)[1].(entity.PasskeyUsed)
	require.True(t, ok)
	assert.Equal(t, nowStored(), used.At)

	var record webauthn.Credential

	require.NoError(t, json.Unmarshal(used.Credential, &record))
	assert.Equal(t, uint32(1), record.Authenticator.SignCount)
}

func TestFinishingTheSameLoginAgainIsAReplay(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.register(t)
	l := f.login(t)
	token, body, _ := f.assertion(t)

	_, err := l.handler.Handle(context.Background(), command.FinishPasskeyLogin{Token: token, Body: body})
	require.NoError(t, err)

	replayed, err := l.handler.Handle(context.Background(), command.FinishPasskeyLogin{Token: token, Body: body})

	require.ErrorIs(t, err, entity.ErrPasskeyChallengeUsed)
	assert.Empty(t, replayed.Token)
	assert.Len(t, f.events(id), 2, "a replay records no second use")
}

func TestADeletedPasskeyCannotSignIn(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.register(t)

	_, err := command.DeletePasskeyHandler{Store: f.store}.Handle(context.Background(),
		command.DeletePasskey{UserID: f.user.ID, ID: id.String()})
	require.NoError(t, err)

	token, body, _ := f.assertion(t)

	_, err = f.login(t).handler.Handle(context.Background(), command.FinishPasskeyLogin{Token: token, Body: body})

	require.ErrorIs(t, err, entity.ErrPasskeyLoginInvalid)
}

func TestAnUnregisteredCredentialCannotSignIn(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	token, body, _ := f.assertion(t)

	_, err := f.login(t).handler.Handle(context.Background(), command.FinishPasskeyLogin{Token: token, Body: body})

	require.ErrorIs(t, err, entity.ErrPasskeyLoginInvalid)
}

func TestAGuardFailureIsNotABadCeremony(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	id := f.register(t)
	l := f.login(t)
	l.guard.burnErr = errGuard

	token, body, _ := f.assertion(t)

	_, err := l.handler.Handle(context.Background(), command.FinishPasskeyLogin{Token: token, Body: body})

	require.ErrorIs(t, err, errGuard)
	require.NotErrorIs(t, err, entity.ErrPasskeyLoginInvalid)
	assert.Len(t, f.events(id), 1, "a login whose burn failed records no use")
}
