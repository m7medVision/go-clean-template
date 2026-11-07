package config_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OIDC_ROLE_MAP is name:role pairs separated by commas.
func TestNewConfig_OIDCRoleMapReadsPairs(t *testing.T) {
	setRequired(t)
	t.Setenv("OIDC_ROLE_MAP", "app-admin:admin,app-user:user")

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"app-admin": "admin", "app-user": "user"}, cfg.OIDC.RoleMap)
}
