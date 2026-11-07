package command_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/m7medvision/go-clean-template/internal/repo/session"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/command"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	_issuer   = "https://sso.example.com"
	_subject  = "idp-subject-1"
	_email    = "alice@example.com"
	_username = "alice"
)

var errClaims = errors.New("claims unreadable")

// fakeSSO is a provider whose sign-in always verifies as identity, or fails with err.
type fakeSSO struct {
	identity entity.SSOIdentity
	err      error
}

func (*fakeSSO) Enabled() bool { return true }

func (*fakeSSO) Begin(context.Context) (authURL, token string, err error) { return "", "", nil }

func (f *fakeSSO) Verify(context.Context, string, string, string) (entity.SSOIdentity, error) {
	return f.identity, f.err
}

type fixture struct {
	store   *memstore.Store
	tokens  *jwt.Manager
	handler command.FinishOIDCLoginHandler
}

func alice(roles ...entity.Role) entity.SSOIdentity {
	return entity.SSOIdentity{Issuer: _issuer, Subject: _subject, Email: _email, Username: _username, Roles: roles}
}

func newFixture(t *testing.T, store repo.EventStore, provider repo.SSOProvider) *fixture {
	t.Helper()

	mem, _ := store.(*memstore.Store) //nolint:errcheck // nil when the test wraps the store
	tokens := jwt.New("test-secret", time.Hour)

	return &fixture{
		store: mem, tokens: tokens,
		handler: command.FinishOIDCLoginHandler{SSO: provider, Store: store, Sessions: session.New(tokens)},
	}
}

// signIn finishes a sign-in the provider verifies.
func (f *fixture) signIn(t *testing.T) (entity.SignedIn, error) {
	t.Helper()

	return f.handler.Handle(t.Context(), command.FinishOIDCLogin{Code: "code", State: "state", Token: "token"})
}

func identity() string { return entity.OIDCIdentity(_issuer, _subject) }

func TestAFirstSignInCreatesTheUser(t *testing.T) {
	t.Parallel()

	f := newFixture(t, memstore.New(entity.EventTypes()), &fakeSSO{identity: alice(entity.RoleAdmin)})

	signedIn, err := f.signIn(t)
	require.NoError(t, err)
	assert.Positive(t, signedIn.Position)

	events := f.store.Events(entity.UserStreamID(signedIn.ID))
	require.Len(t, events, 1)

	created, ok := events[0].(entity.UserRegisteredViaOIDC)
	require.True(t, ok)
	assert.Equal(t, _username, created.Username)
	assert.Equal(t, _email, created.Email)
	assert.Equal(t, _issuer, created.Issuer)
	assert.Equal(t, _subject, created.Subject)

	owner, err := f.store.ClaimOwner(t.Context(), entity.ClaimOIDCIdentity, identity())
	require.NoError(t, err)
	assert.Equal(t, signedIn.ID, owner)

	claims, err := f.tokens.Parse(signedIn.Token)
	require.NoError(t, err)
	assert.Equal(t, signedIn.ID, claims.UserID)
	assert.Equal(t, []string{"admin"}, claims.Roles)
}

func TestAKnownIdentitySignsInAsItsUser(t *testing.T) {
	t.Parallel()

	f := newFixture(t, memstore.New(entity.EventTypes()), &fakeSSO{identity: alice()})

	first, err := f.signIn(t)
	require.NoError(t, err)

	again, err := f.signIn(t)
	require.NoError(t, err)

	assert.Equal(t, first.ID, again.ID)
	assert.Zero(t, again.Position, "a known identity writes nothing")
	assert.Len(t, f.store.Events(entity.UserStreamID(first.ID)), 1)
}

// racingStore says once that the identity is unclaimed though another sign-in holds it, as when two
// first sign-ins of one identity race.
type racingStore struct {
	*memstore.Store

	lied atomic.Bool
}

func (s *racingStore) ClaimOwner(ctx context.Context, scope, value string) (string, error) {
	if s.lied.CompareAndSwap(false, true) {
		return "", entity.ErrClaimNotFound
	}

	return s.Store.ClaimOwner(ctx, scope, value)
}

func TestARacingFirstSignInSignsInAsTheWinner(t *testing.T) {
	t.Parallel()

	mem := memstore.New(entity.EventTypes())

	winner, err := newFixture(t, mem, &fakeSSO{identity: alice()}).signIn(t)
	require.NoError(t, err)

	loser, err := newFixture(t, &racingStore{Store: mem}, &fakeSSO{identity: alice()}).signIn(t)
	require.NoError(t, err)
	assert.Equal(t, winner.ID, loser.ID)
}

func TestAFirstSignInWithATakenEmailIsAnAccountConflict(t *testing.T) {
	t.Parallel()

	mem := memstore.New(entity.EventTypes())
	local, claims := entity.NewUserAggregate("local-user", "someone", _email, "hash", time.Now())
	_, err := mem.Save(context.Background(), local, claims...)
	require.NoError(t, err)

	_, err = newFixture(t, mem, &fakeSSO{identity: alice()}).signIn(t)

	require.ErrorIs(t, err, entity.ErrAccountConflict)
}

// failingStore cannot read claims.
type failingStore struct{ *memstore.Store }

func (failingStore) ClaimOwner(context.Context, string, string) (string, error) { return "", errClaims }

func TestAClaimReadFailureIsReturned(t *testing.T) {
	t.Parallel()

	_, err := newFixture(t, failingStore{memstore.New(entity.EventTypes())}, &fakeSSO{identity: alice()}).signIn(t)

	require.ErrorIs(t, err, errClaims)
}

func TestAnInvalidSignInWritesNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t, memstore.New(entity.EventTypes()), &fakeSSO{err: entity.ErrOIDCCeremonyInvalid})

	_, err := f.signIn(t)
	require.ErrorIs(t, err, entity.ErrOIDCCeremonyInvalid)

	_, err = f.store.ClaimOwner(t.Context(), entity.ClaimOIDCIdentity, identity())
	require.ErrorIs(t, err, entity.ErrClaimNotFound)
}
