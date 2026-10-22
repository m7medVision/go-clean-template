package entity_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
)

func TestRole_Valid(t *testing.T) {
	t.Parallel()

	assert.True(t, entity.RoleAdmin.Valid())
	assert.True(t, entity.RoleUser.Valid())
	assert.False(t, entity.Role("superuser").Valid())
	assert.False(t, entity.Role("").Valid())
}

func TestParseRoles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []entity.Role
	}{
		{"known roles keep their order", []string{"user", "admin"}, []entity.Role{entity.RoleUser, entity.RoleAdmin}},
		{"unknown roles are dropped", []string{"admin", "superuser", ""}, []entity.Role{entity.RoleAdmin}},
		{"duplicates collapse", []string{"user", "user"}, []entity.Role{entity.RoleUser}},
		{"nothing known is empty", []string{"superuser"}, []entity.Role{}},
		{"nil is empty", nil, []entity.Role{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, entity.ParseRoles(tt.in))
		})
	}
}

func TestPrincipal_Has(t *testing.T) {
	t.Parallel()

	admin := entity.Principal{UserID: "u1", Roles: []entity.Role{entity.RoleUser, entity.RoleAdmin}}
	user := entity.Principal{UserID: "u2", Roles: []entity.Role{entity.RoleUser}}
	none := entity.Principal{UserID: "u3"}

	assert.True(t, admin.Has(entity.RoleAdmin))
	assert.False(t, user.Has(entity.RoleAdmin))
	assert.True(t, user.Has(entity.RoleUser))
	assert.False(t, none.Has(entity.RoleUser))
}

func TestLocalRoles(t *testing.T) {
	t.Parallel()

	// Sign-ins the app verifies itself never grant admin; only an identity provider can.
	assert.Equal(t, []entity.Role{entity.RoleUser}, entity.LocalRoles())
}

func TestRoleNames(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"admin", "user"}, entity.RoleNames([]entity.Role{entity.RoleAdmin, entity.RoleUser}))
	assert.Equal(t, []string{}, entity.RoleNames(nil))
}
