package user_test

import (
	"context"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rows is a users table and its identities holding legacy rows; any other query panics on the nil
// embedded interface.
type rows struct {
	queries.Querier

	users      []queries.User
	identities []queries.UserIdentity
	err        error
}

func (r *rows) ListEveryUser(context.Context) ([]queries.User, error) { return r.users, r.err }

func (r *rows) ListEveryIdentity(context.Context) ([]queries.UserIdentity, error) {
	return r.identities, nil
}

func row(id, name string) queries.User {
	return queries.User{ID: id, Username: name, Email: name + "@example.com", PasswordHash: "h", CreatedAt: at(), UpdatedAt: at()}
}

func TestEachUserRowBecomesAStreamHoldingUserImportedWithItsClaims(t *testing.T) {
	t.Parallel()

	imports, err := user.Imports(context.Background(), &rows{
		users:      []queries.User{row("u1", "jo"), row("u2", "al")},
		identities: []queries.UserIdentity{{Issuer: "iss", Subject: "sub", UserID: "u2", CreatedAt: at()}},
	})
	require.NoError(t, err)
	require.Len(t, imports, 2)

	assert.Equal(t, entity.UserStreamID("u1"), imports[0].Aggregate.StreamID())
	assert.Equal(t, []entity.Event{entity.UserImported{
		UserID: "u1", Username: "jo", Email: "jo@example.com", PasswordHash: "h", CreatedAt: at(), UpdatedAt: at(),
	}}, imports[0].Aggregate.Pending())

	claimed := map[string]string{}
	for _, c := range imports[1].Claims {
		claimed[c.Scope] = c.Value
		assert.Equal(t, "u2", c.Owner)
	}

	assert.Equal(t, map[string]string{
		entity.ClaimUserEmail:    "al@example.com",
		entity.ClaimUserUsername: "al",
		entity.ClaimOIDCIdentity: entity.OIDCIdentity("iss", "sub"),
	}, claimed)
}

func TestTheUserImporterReturnsAReadFailure(t *testing.T) {
	t.Parallel()

	_, err := user.Imports(context.Background(), &rows{err: errWrite})

	require.ErrorIs(t, err, errWrite)
}
