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
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/command"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/oidctest"
	"github.com/m7medvision/go-clean-template/internal/usecase/oidc/sso"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _stateTTL = 10 * time.Minute

var errClaims = errors.New("claims unreadable")

type fixture struct {
	idp     *oidctest.IDP
	sso     *sso.SSO
	store   *memstore.Store
	tokens  *jwt.Manager
	handler command.FinishOIDCLoginHandler
}

func newFixture(t *testing.T, store repo.EventStore) *fixture {
	t.Helper()

	idp := oidctest.New(t)

	state, err := jwt.NewScoped("test-secret", "oidc-state", _stateTTL)
	require.NoError(t, err)

	s := sso.New(state, &sso.Config{
		Issuer: idp.Issuer(), ClientID: oidctest.ClientID, ClientSecret: oidctest.ClientSecret,
		RedirectURL: oidctest.RedirectURL, RolesClaim: "realm_access.roles",
		RoleMap: map[string]string{"app-admin": "admin"},
	})

	mem, _ := store.(*memstore.Store) //nolint:errcheck // nil when the test wraps the store
	tokens := jwt.New("test-secret", time.Hour)

	return &fixture{
		idp: idp, sso: s, store: mem, tokens: tokens,
		handler: command.FinishOIDCLoginHandler{SSO: s, Store: store, Tokens: tokens},
	}
}

// signIn begins a sign-in, signs in at the provider with claims, and finishes it.
func (f *fixture) signIn(t *testing.T, claims map[string]any) (command.SignedIn, error) {
	t.Helper()

	authURL, token, err := f.sso.Begin(t.Context())
	require.NoError(t, err)

	code, state := f.idp.Authorize(authURL, claims)

	return f.handler.Handle(t.Context(), command.FinishOIDCLogin{Code: code, State: state, Token: token})
}

func (f *fixture) identity() string { return entity.OIDCIdentity(f.idp.Issuer(), oidctest.Subject) }

func TestAFirstSignInCreatesTheUser(t *testing.T) {
	t.Parallel()

	f := newFixture(t, memstore.New(entity.EventTypes()))

	signedIn, err := f.signIn(t, map[string]any{"realm_access": map[string]any{"roles": []string{"app-admin"}}})
	require.NoError(t, err)
	assert.Positive(t, signedIn.Position)

	events := f.store.Events(entity.UserStreamID(signedIn.ID))
	require.Len(t, events, 1)

	created, ok := events[0].(entity.UserRegisteredViaOIDC)
	require.True(t, ok)
	assert.Equal(t, oidctest.Username, created.Username)
	assert.Equal(t, oidctest.Email, created.Email)
	assert.Equal(t, f.idp.Issuer(), created.Issuer)
	assert.Equal(t, oidctest.Subject, created.Subject)

	owner, err := f.store.ClaimOwner(t.Context(), entity.ClaimOIDCIdentity, f.identity())
	require.NoError(t, err)
	assert.Equal(t, signedIn.ID, owner)

	claims, err := f.tokens.Parse(signedIn.Token)
	require.NoError(t, err)
	assert.Equal(t, signedIn.ID, claims.UserID)
	assert.Equal(t, []string{"admin"}, claims.Roles)
}

func TestAKnownIdentitySignsInAsItsUser(t *testing.T) {
	t.Parallel()

	f := newFixture(t, memstore.New(entity.EventTypes()))

	first, err := f.signIn(t, nil)
	require.NoError(t, err)

	again, err := f.signIn(t, nil)
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
	f := newFixture(t, mem)

	winner, err := f.signIn(t, nil)
	require.NoError(t, err)

	racing := newFixture(t, &racingStore{Store: mem})
	racing.idp = f.idp
	racing.sso = f.sso
	racing.handler.SSO = f.sso

	loser, err := racing.signIn(t, nil)
	require.NoError(t, err)
	assert.Equal(t, winner.ID, loser.ID)
}

func TestAFirstSignInWithATakenEmailIsAnAccountConflict(t *testing.T) {
	t.Parallel()

	mem := memstore.New(entity.EventTypes())
	local, claims := entity.NewUserAggregate("local-user", "someone", oidctest.Email, "hash", time.Now())
	_, err := mem.Save(context.Background(), local, claims...)
	require.NoError(t, err)

	_, err = newFixture(t, mem).signIn(t, nil)

	require.ErrorIs(t, err, entity.ErrAccountConflict)
}

// failingStore cannot read claims.
type failingStore struct{ *memstore.Store }

func (failingStore) ClaimOwner(context.Context, string, string) (string, error) { return "", errClaims }

func TestAClaimReadFailureIsReturned(t *testing.T) {
	t.Parallel()

	_, err := newFixture(t, failingStore{memstore.New(entity.EventTypes())}).signIn(t, nil)

	require.ErrorIs(t, err, errClaims)
}

func TestAnInvalidSignInWritesNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t, memstore.New(entity.EventTypes()))

	_, err := f.signIn(t, map[string]any{"email": nil})
	require.ErrorIs(t, err, entity.ErrOIDCCeremonyInvalid)

	_, err = f.store.ClaimOwner(t.Context(), entity.ClaimOIDCIdentity, f.identity())
	require.ErrorIs(t, err, entity.ErrClaimNotFound)
}
