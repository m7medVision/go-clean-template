package config_test

import (
	"testing"

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
}

// The links the app builds join paths to APP_BASE_URL, so a trailing slash would double.
func TestNewConfig_BaseURLLosesItsTrailingSlash(t *testing.T) {
	setRequired(t)
	t.Setenv("APP_BASE_URL", "https://app.example.com/")

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	assert.Equal(t, "https://app.example.com", cfg.App.BaseURL)
}
