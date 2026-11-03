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
		"PG_POOL_MAX": "2", "PG_URL": "postgres://localhost/db", "JWT_SECRET": "s",
	} {
		t.Setenv(k, v)
	}

	// make exports .env.example into every target, so clear what these tests read as defaults.
	for _, k := range []string{
		"JWT_TOKEN_EXPIRY",
		"GRPC_PORT",
		"CQRS_CONSISTENCY_TIMEOUT", "EVENTSTORE_SNAPSHOT_EVERY", "PROJECTION_POLL_INTERVAL", "PROJECTION_BATCH_SIZE",
		"OIDC_ISSUER", "OIDC_ROLES_CLAIM", "OIDC_ROLE_MAP",
	} {
		t.Setenv(k, "") // restores the variable after the test
		require.NoError(t, os.Unsetenv(k))
	}
}

//nolint:paralleltest // t.Setenv (in setRequired) cannot run in parallel.
func TestNewConfig_TokenExpiryDefault(t *testing.T) {
	setRequired(t)

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	// Roles are fixed when a token is issued, so its lifetime bounds how long a revoked role lingers.
	assert.Equal(t, 15*time.Minute, cfg.JWT.TokenExpiry)
}

//nolint:paralleltest // t.Setenv (in setRequired) cannot run in parallel.
func TestNewConfig_CQRSDefaults(t *testing.T) {
	setRequired(t)

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	assert.Equal(t, 2*time.Second, cfg.CQRS.ConsistencyTimeout)
	assert.Equal(t, 100, cfg.CQRS.SnapshotEvery)
	assert.Equal(t, time.Second, cfg.CQRS.ProjectionPollInterval)
	assert.Equal(t, 100, cfg.CQRS.ProjectionBatchSize)
}
