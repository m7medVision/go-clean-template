package user_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/memstore"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func row(id, name string) entity.User {
	return entity.User{ID: id, Username: name, Email: name + "@example.com", PasswordHash: "h", CreatedAt: at(), UpdatedAt: at()}
}

func TestImportUsersGivesEachRowItsStreamAndClaims(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())
	identities := map[string][]entity.UserIdentity{"u2": {{Issuer: "iss", Subject: "sub", CreatedAt: at()}}}

	require.NoError(t, user.ImportUsers(context.Background(), store, []entity.User{row("u1", "jo"), row("u2", "al")}, identities))

	assert.Equal(t, []entity.Event{entity.UserImported{
		UserID: "u1", Username: "jo", Email: "jo@example.com", PasswordHash: "h", CreatedAt: at(), UpdatedAt: at(),
	}}, store.Events(entity.UserStreamID("u1")))

	owner, err := store.ClaimOwner(context.Background(), entity.ClaimUserEmail, "al@example.com")
	require.NoError(t, err)
	assert.Equal(t, "u2", owner)

	owner, err = store.ClaimOwner(context.Background(), entity.ClaimOIDCIdentity, entity.OIDCIdentity("iss", "sub"))
	require.NoError(t, err)
	assert.Equal(t, "u2", owner)
}

func TestImportUsersSkipsAUserThatHasAStreamAlready(t *testing.T) {
	t.Parallel()

	store := memstore.New(entity.EventTypes())
	existing, claims := entity.NewUserAggregate("u1", "jo", "jo@example.com", "h", at())
	_, err := store.Save(context.Background(), existing, claims...)
	require.NoError(t, err)

	require.NoError(t, user.ImportUsers(context.Background(), store, []entity.User{row("u1", "jo")}, nil))

	assert.Len(t, store.Events(entity.UserStreamID("u1")), 1)
}

func TestTheImporterIsTheUserDomains(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "user", user.NewImporter(nil, nil).Domain())
}
