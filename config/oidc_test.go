package config_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig_OIDC(t *testing.T) {
	setRequired(t)
	t.Setenv("OIDC_ISSUER", "http://keycloak.lvh.me:8180/realms/app")
	t.Setenv("OIDC_ROLE_MAP", "app-admin:admin,app-user:user")

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	assert.Equal(t, "http://keycloak.lvh.me:8180/realms/app", cfg.OIDC.Issuer)
	assert.Equal(t, "realm_access.roles", cfg.OIDC.RolesClaim, "the default is Keycloak's claim")
	assert.Equal(t, map[string]string{"app-admin": "admin", "app-user": "user"}, cfg.OIDC.RoleMap)
}

//nolint:paralleltest // t.Setenv (in setRequired) cannot run in parallel.
func TestNewConfig_OIDCOffByDefault(t *testing.T) {
	setRequired(t)

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	assert.Empty(t, cfg.OIDC.Issuer)
}
