package command_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/descope/virtualwebauthn"
	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/ceremony"
	"github.com/m7medvision/go-clean-template/internal/repo/ceremony/ceremonytest"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/m7medvision/go-clean-template/internal/usecase/passkey/command"
	"github.com/stretchr/testify/require"
)

var errGuard = errors.New("guard failed")

func now() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 999, time.UTC) }

func nowStored() time.Time { return now().Truncate(time.Microsecond) }

func newCeremony(t *testing.T) *ceremony.Ceremony {
	t.Helper()

	c, err := ceremony.New(ceremonytest.SigningKey, &ceremony.Config{
		RPID: ceremonytest.RPID, RPName: ceremonytest.RPName, RPOrigins: []string{ceremonytest.RPOrigin},
		Timeout: ceremonytest.CeremonyTimeout, ReauthMaxAge: ceremonytest.ReauthMaxAge,
	})
	require.NoError(t, err)

	return c
}

func newUser() entity.User {
	return entity.User{ID: uuid.NewString(), Username: "alice", Email: "alice@example.com"}
}

// fixture is a store, a ceremony, and the user and authenticator a test registers with.
type fixture struct {
	store    *memstore.Store
	ceremony *ceremony.Ceremony
	user     entity.User
	cred     virtualwebauthn.Credential
	auth     virtualwebauthn.Authenticator
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	user := newUser()
	handle := uuid.MustParse(user.ID)

	return &fixture{
		store:    memstore.New(entity.EventTypes()),
		ceremony: newCeremony(t),
		user:     user,
		cred:     virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2),
		auth:     virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{UserHandle: handle[:]}),
	}
}

// attestation begins a registration for the fixture's user and answers it with its credential.
func (f *fixture) attestation(t *testing.T) (token string, body []byte) {
	t.Helper()

	started, err := f.ceremony.BeginRegistration(&f.user, nil)
	require.NoError(t, err)

	return started.Token, ceremonytest.Attest(t, started.Options, f.cred)
}

// register registers the fixture's credential through FinishPasskeyRegistration.
func (f *fixture) register(t *testing.T) entity.PasskeyID {
	t.Helper()

	token, body := f.attestation(t)

	_, err := command.FinishPasskeyRegistrationHandler{Ceremony: f.ceremony, Store: f.store, Now: now}.Handle(
		context.Background(), command.FinishPasskeyRegistration{UserID: f.user.ID, Token: token, Name: "Laptop", Body: body})
	require.NoError(t, err)

	return entity.PasskeyID(f.cred.ID)
}

func (f *fixture) events(id entity.PasskeyID) []entity.Event {
	return f.store.Events(entity.PasskeyStreamID(id))
}

// burnStore is an in-memory webauthn_used_challenges: BurnChallenge succeeds once per hash.
type burnStore struct {
	mu        sync.Mutex
	used      map[string]time.Time
	burnErr   error
	expireErr error
}

func newBurnStore() *burnStore { return &burnStore{used: map[string]time.Time{}} }

func (b *burnStore) BurnChallenge(_ context.Context, hash []byte, expiresAt time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.burnErr != nil {
		return b.burnErr
	}

	if _, ok := b.used[string(hash)]; ok {
		return entity.ErrPasskeyChallengeUsed
	}

	b.used[string(hash)] = expiresAt

	return nil
}

func (b *burnStore) DeleteExpiredChallenges(context.Context) error { return b.expireErr }

// inline runs a transaction's work as it is: the memstore has no transactions to join.
type inline struct{}

func (inline) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }
