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

	tests := []struct {
		name      string
		roles     []string
		wantRoles []entity.Role
	}{
		{"admin token", []string{"admin", "user"}, []entity.Role{entity.RoleAdmin, entity.RoleUser}},
		{"unknown roles are dropped", []string{"superuser", "user"}, []entity.Role{entity.RoleUser}},
		{"no roles claim", nil, []entity.Role{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			before := time.Now().Truncate(time.Second)

			token, err := tokens.GenerateToken("user-1", tt.roles)
			require.NoError(t, err)

			c, err := authn.ReadToken(tokens, token)
			require.NoError(t, err)

			assert.Equal(t, entity.Principal{UserID: "user-1", Roles: tt.wantRoles}, c.Principal)
			assert.False(t, c.IssuedAt.Before(before), "issued at %s, before %s", c.IssuedAt, before)
			assert.False(t, c.IssuedAt.After(time.Now()), "issued in the future: %s", c.IssuedAt)
		})
	}
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

func TestCallerRoundTrip(t *testing.T) {
	t.Parallel()

	want := authn.Caller{
		UserID: "user-1", Roles: []entity.Role{entity.RoleUser},
		IssuedAt: time.Unix(1_700_000_000, 0),
	}

	ctx := authn.WithCaller(t.Context(), want)

	got, ok := authn.CallerFrom(ctx)
	require.True(t, ok)
	assert.Equal(t, want, got)
	assert.Equal(t, want, authn.MustCaller(ctx))
}

func TestNoCaller(t *testing.T) {
	t.Parallel()

	_, ok := authn.CallerFrom(t.Context())
	assert.False(t, ok)

	assert.Panics(t, func() { authn.MustCaller(t.Context()) },
		"an operation that reads a caller it never required is a programming error")
}
