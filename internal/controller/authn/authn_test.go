package authn_test

import (
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/controller/authn"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/m7medvision/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadToken(t *testing.T) {
	t.Parallel()

	tokens := jwt.New("test-secret", time.Hour)
	before := time.Now().Truncate(time.Second)

	token, err := tokens.GenerateToken("user-1", []string{"admin", "superuser", "user"})
	require.NoError(t, err)

	c, err := authn.ReadToken(tokens, token)
	require.NoError(t, err)

	assert.Equal(t, entity.Principal{UserID: "user-1", Roles: []entity.Role{entity.RoleAdmin, entity.RoleUser}}, c.Principal,
		"a role the app does not know grants nothing")
	assert.False(t, c.IssuedAt.Before(before), "issued at %s, before %s", c.IssuedAt, before)
	assert.False(t, c.IssuedAt.After(time.Now()), "issued in the future: %s", c.IssuedAt)
}

func TestReadTokenRejects(t *testing.T) {
	t.Parallel()

	tokens := jwt.New("test-secret", time.Hour)

	otherKey, err := jwt.New("other-secret", time.Hour).GenerateToken("user-1", nil)
	require.NoError(t, err)

	expired, err := jwt.New("test-secret", -time.Minute).GenerateToken("user-1", nil)
	require.NoError(t, err)

	for name, token := range map[string]string{"garbage": "not-a-token", "another key": otherKey, "expired": expired} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := authn.ReadToken(tokens, token)
			require.Error(t, err)
		})
	}
}

func TestNoCaller(t *testing.T) {
	t.Parallel()

	_, ok := authn.CallerFrom(t.Context())
	assert.False(t, ok)

	assert.Panics(t, func() { authn.MustCaller(t.Context()) },
		"an operation that reads a caller it never required is a programming error")
}
