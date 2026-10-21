package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequired(t *testing.T) {
	t.Helper()

	for k, v := range map[string]string{
		"APP_NAME": "app", "APP_VERSION": "1", "HTTP_PORT": "8080", "LOG_LEVEL": "info",
		"PG_POOL_MAX": "2", "PG_URL": "postgres://localhost/db", "GRPC_PORT": "8081", "JWT_SECRET": "s",
	} {
		t.Setenv(k, v)
	}

	// make exports .env.example into every target, so clear what these tests read as defaults.
	for _, k := range []string{"JWT_TOKEN_EXPIRY", "OIDC_ISSUER", "OIDC_ROLES_CLAIM", "OIDC_ROLE_MAP"} {
		t.Setenv(k, "") // restores the variable after the test
		require.NoError(t, os.Unsetenv(k))
	}
}

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

//nolint:paralleltest // t.Setenv (in setRequired) cannot run in parallel.
func TestNewConfig_TokenExpiryDefault(t *testing.T) {
	setRequired(t)

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	// Roles are fixed when a token is issued, so its lifetime bounds how long a revoked role lingers.
	assert.Equal(t, 15*time.Minute, cfg.JWT.TokenExpiry)
}
