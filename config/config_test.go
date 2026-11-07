package config_test

import (
	"os"
	"testing"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequired(t *testing.T) {
	t.Helper()

	for k, v := range map[string]string{
		"APP_NAME": "app", "APP_VERSION": "1", "LOG_LEVEL": "info",
		"PG_POOL_MAX": "2", "PG_URL": "postgres://localhost/db",
	} {
		t.Setenv(k, v)
	}
}

// The worker reads neither HTTP_PORT nor JWT_SECRET, so it starts without them; the app checks them
// itself.
func TestNewConfig_TheAppOnlySettingsAreOptional(t *testing.T) {
	setRequired(t)

	// Setenv restores each variable after the test; Unsetenv removes it for the test.
	t.Setenv("HTTP_PORT", "")
	t.Setenv("JWT_SECRET", "")
	require.NoError(t, os.Unsetenv("HTTP_PORT"))
	require.NoError(t, os.Unsetenv("JWT_SECRET"))

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	assert.Empty(t, cfg.HTTP.Port)
	assert.Empty(t, cfg.JWT.Secret)
}

// The links the app builds join paths to APP_BASE_URL, so a trailing slash would double.
func TestNewConfig_BaseURLLosesItsTrailingSlash(t *testing.T) {
	setRequired(t)
	t.Setenv("APP_BASE_URL", "https://app.example.com/")

	cfg, err := config.NewConfig()
	require.NoError(t, err)

	assert.Equal(t, "https://app.example.com", cfg.App.BaseURL)
}
