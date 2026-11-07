package query_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/descope/virtualwebauthn"
	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/config"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/ceremony"
	"github.com/m7medvision/go-clean-template/internal/repo/ceremony/ceremonytest"
	"github.com/m7medvision/go-clean-template/internal/repo/repomock"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errRead = errors.New("read failed")

func newCeremony(t *testing.T) *ceremony.Ceremony {
	t.Helper()

	c, err := ceremony.New(ceremonytest.SigningKey, config.WebAuthn{
		RPID: ceremonytest.RPID, RPName: ceremonytest.RPName, RPOrigins: []string{ceremonytest.RPOrigin},
		CeremonyTimeout: ceremonytest.CeremonyTimeout, ReauthMaxAge: ceremonytest.ReauthMaxAge,
	}, time.Now)
	require.NoError(t, err)

	return c
}

type reads struct {
	users    *repomock.MockUserReadModel
	passkeys *repomock.MockPasskeyReadModel
}

func newReads(t *testing.T) reads {
	t.Helper()

	ctrl := gomock.NewController(t)

	return reads{users: repomock.NewMockUserReadModel(ctrl), passkeys: repomock.NewMockPasskeyReadModel(ctrl)}
}

func newUser() entity.User {
	return entity.User{ID: uuid.NewString(), Username: "alice", Email: "alice@example.com"}
}

func TestBeginPasskeyRegistrationStartsACeremonyTheBrowserCanAnswer(t *testing.T) {
	t.Parallel()

	r := newReads(t)
	user := newUser()
	r.users.EXPECT().ByID(gomock.Any(), user.ID).Return(user, nil)
	r.passkeys.EXPECT().ListByUser(gomock.Any(), user.ID).Return(nil, nil)

	h := query.BeginPasskeyRegistrationHandler{Ceremony: newCeremony(t), Users: r.users, Passkeys: r.passkeys}

	started, err := h.Handle(context.Background(), query.BeginPasskeyRegistration{UserID: user.ID, AuthTime: time.Now()})
	require.NoError(t, err)
	require.NotEmpty(t, started.Token)

	parsed, err := virtualwebauthn.ParseAttestationOptions(string(started.Options))
	require.NoError(t, err)
	assert.Equal(t, user.Email, parsed.UserName)
}

func TestBeginPasskeyRegistrationRefusesAStaleSignInBeforeReadingAnything(t *testing.T) {
	t.Parallel()

	r := newReads(t)
	h := query.BeginPasskeyRegistrationHandler{Ceremony: newCeremony(t), Users: r.users, Passkeys: r.passkeys}

	_, err := h.Handle(context.Background(), query.BeginPasskeyRegistration{
		UserID: uuid.NewString(), AuthTime: time.Now().Add(-ceremonytest.ReauthMaxAge - time.Minute),
	})

	require.ErrorIs(t, err, entity.ErrPasskeyReauthRequired)
}

func TestBeginPasskeyRegistrationKeepsAReadFailure(t *testing.T) {
	t.Parallel()

	user := newUser()

	r := newReads(t)
	r.users.EXPECT().ByID(gomock.Any(), user.ID).Return(entity.User{}, entity.ErrUserNotFound)

	_, err := query.BeginPasskeyRegistrationHandler{Ceremony: newCeremony(t), Users: r.users, Passkeys: r.passkeys}.Handle(
		context.Background(), query.BeginPasskeyRegistration{UserID: user.ID, AuthTime: time.Now()})
	require.ErrorIs(t, err, entity.ErrUserNotFound)

	r = newReads(t)
	r.users.EXPECT().ByID(gomock.Any(), user.ID).Return(user, nil)
	r.passkeys.EXPECT().ListByUser(gomock.Any(), user.ID).Return(nil, errRead)

	_, err = query.BeginPasskeyRegistrationHandler{Ceremony: newCeremony(t), Users: r.users, Passkeys: r.passkeys}.Handle(
		context.Background(), query.BeginPasskeyRegistration{UserID: user.ID, AuthTime: time.Now()})
	require.ErrorIs(t, err, errRead)
}

func TestBeginPasskeyLoginStartsADiscoverableCeremony(t *testing.T) {
	t.Parallel()

	started, err := query.BeginPasskeyLoginHandler{Ceremony: newCeremony(t)}.Handle(context.Background(), query.BeginPasskeyLogin{})
	require.NoError(t, err)
	require.NotEmpty(t, started.Token)

	parsed, err := virtualwebauthn.ParseAssertionOptions(string(started.Options))
	require.NoError(t, err)
	assert.Empty(t, parsed.AllowCredentials)
}

func TestListPasskeysReadsTheUsersPasskeys(t *testing.T) {
	t.Parallel()

	r := newReads(t)
	want := []entity.Passkey{{ID: entity.PasskeyID{1}, Name: "Laptop"}}
	r.passkeys.EXPECT().ListByUser(gomock.Any(), "u1").Return(want, nil)

	got, err := query.ListPasskeysHandler{Passkeys: r.passkeys}.Handle(context.Background(), query.ListPasskeys{UserID: "u1"})

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetPasskeyReadsOneOfTheUsersPasskeys(t *testing.T) {
	t.Parallel()

	r := newReads(t)
	want := entity.Passkey{ID: entity.PasskeyID{1, 2, 3, 4}, Name: "Laptop"}
	r.passkeys.EXPECT().Get(gomock.Any(), "u1", entity.PasskeyID{1, 2, 3, 4}).Return(want, nil)

	got, err := query.GetPasskeyHandler{Passkeys: r.passkeys}.Handle(context.Background(), query.GetPasskey{UserID: "u1", ID: "AQIDBA"})

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetPasskeyOfAnIDThatIsNotBase64URLIsNotFound(t *testing.T) {
	t.Parallel()

	_, err := query.GetPasskeyHandler{Passkeys: newReads(t).passkeys}.Handle(context.Background(),
		query.GetPasskey{UserID: "u1", ID: "not base64!"})

	require.ErrorIs(t, err, entity.ErrPasskeyNotFound)
}
