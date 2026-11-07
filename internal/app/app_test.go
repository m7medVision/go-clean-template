package app

import (
	"testing"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/stretchr/testify/require"
)

// The worker starts without JWT_SECRET and VALKEY_URL, so config leaves them optional; the app
// refuses to start without either, before it opens anything.
func TestTheAppRefusesToStartWithoutAJWTSecretOrAValkeyURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *config.Config
		want error
	}{
		{"no JWT secret", &config.Config{HTTP: config.HTTP{Port: "8080"}, Valkey: config.Valkey{URL: "valkey://localhost:6379/0"}}, ErrNoJWTSecret},
		{"no Valkey URL", &config.Config{HTTP: config.HTTP{Port: "8080"}, JWT: config.JWT{Secret: "secret"}}, ErrNoValkeyURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, Run(t.Context(), tt.cfg), tt.want)
		})
	}
}
