package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errWrite = errors.New("write failed")

// fakeQuerier records the user writes; any other query panics on the nil embedded interface.
type fakeQuerier struct {
	queries.Querier

	users      []queries.InsertUserRowParams
	identities []queries.InsertIdentityRowParams
	err        error
}

//nolint:gocritic // the signature is sqlc's Querier, which takes its params by value
func (f *fakeQuerier) InsertUserRow(_ context.Context, arg queries.InsertUserRowParams) error {
	f.users = append(f.users, arg)

	return f.err
}

func (f *fakeQuerier) InsertIdentityRow(_ context.Context, arg queries.InsertIdentityRowParams) error {
	f.identities = append(f.identities, arg)

	return f.err
}

func at() time.Time { return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC) }

func project(t *testing.T, q *fakeQuerier, e entity.Event) error {
	t.Helper()

	return user.Project(context.Background(), q, &entity.RecordedEvent{Position: 1, StreamID: "user-u1", Event: e})
}

func TestUserRegisteredInsertsTheUser(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}

	require.NoError(t, project(t, q, entity.UserRegistered{
		UserID: "u1", Username: "jo", Email: "jo@example.com", PasswordHash: "h", At: at(),
	}))

	assert.Equal(t, []queries.InsertUserRowParams{{
		ID: "u1", Username: "jo", Email: "jo@example.com", PasswordHash: "h", CreatedAt: at(), UpdatedAt: at(),
	}}, q.users)
	assert.Empty(t, q.identities)
}

func TestUserRegisteredViaOIDCInsertsTheUserAndTheIdentity(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}

	require.NoError(t, project(t, q, entity.UserRegisteredViaOIDC{
		UserID: "u1", Username: "jo", Email: "jo@example.com", Issuer: "iss", Subject: "sub", At: at(),
	}))

	assert.Equal(t, []queries.InsertUserRowParams{{
		ID: "u1", Username: "jo", Email: "jo@example.com", CreatedAt: at(), UpdatedAt: at(),
	}}, q.users)
	assert.Equal(t, []queries.InsertIdentityRowParams{{Issuer: "iss", Subject: "sub", UserID: "u1", CreatedAt: at()}},
		q.identities)
}

func TestUserImportedInsertsTheUserAndEachIdentity(t *testing.T) {
	t.Parallel()

	q := &fakeQuerier{}
	later := at().Add(time.Hour)

	require.NoError(t, project(t, q, entity.UserImported{
		UserID: "u1", Username: "jo", Email: "jo@example.com", PasswordHash: "h", CreatedAt: at(), UpdatedAt: later,
		Identities: []entity.UserIdentity{{Issuer: "a", Subject: "1", CreatedAt: at()}, {Issuer: "b", Subject: "2", CreatedAt: at()}},
	}))

	assert.Equal(t, []queries.InsertUserRowParams{{
		ID: "u1", Username: "jo", Email: "jo@example.com", PasswordHash: "h", CreatedAt: at(), UpdatedAt: later,
	}}, q.users)
	assert.Len(t, q.identities, 2)
}

func TestTheUserProjectorReturnsAWriteFailure(t *testing.T) {
	t.Parallel()

	err := project(t, &fakeQuerier{err: errWrite}, entity.UserRegistered{UserID: "u1"})

	require.ErrorIs(t, err, errWrite)
}

func TestTheUserProjectorHasACaseForEveryEventItHandles(t *testing.T) {
	t.Parallel()

	for _, e := range user.NewProjector(nil).Handles() {
		require.NotErrorIs(t, project(t, &fakeQuerier{}, e), entity.ErrUnhandledEvent, e.EventType())
	}

	require.ErrorIs(t, project(t, &fakeQuerier{}, otherDomainEvent()), entity.ErrUnhandledEvent)
}

func TestTheUserProjectorOwnsTheUserTables(t *testing.T) {
	t.Parallel()

	p := user.NewProjector(nil)

	assert.Equal(t, "user", p.Name())
	assert.Equal(t, []string{"user_identities", "users"}, p.Tables())
}

func otherDomainEvent() entity.Event { return entity.TaskDeleted{} }
