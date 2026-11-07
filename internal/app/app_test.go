package app

import (
	"testing"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/stretchr/testify/require"
)

// The worker starts without JWT_SECRET, so config leaves it optional; the app refuses to start
// without it, before it opens anything.
func TestTheAppRefusesToStartWithoutAJWTSecret(t *testing.T) {
	t.Parallel()

	err := Run(t.Context(), &config.Config{HTTP: config.HTTP{Port: "8080"}})

	require.ErrorIs(t, err, ErrNoJWTSecret)
}
