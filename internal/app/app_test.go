package app

import (
	"net/netip"
	"testing"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/stretchr/testify/require"
)

// The worker starts without JWT_SECRET and VALKEY_URL, so config leaves them optional; the app
// refuses to start without either, or with a TRUSTED_PROXY that is no address, before it opens
// anything.
const _mongoURL = "mongodb://localhost:27017/app"

func TestTheAppRefusesToStartWithoutItsSettings(t *testing.T) {
	t.Parallel()

	_, errBadProxy := netip.ParseAddr("not-an-address")
	require.Error(t, errBadProxy)

	tests := []struct {
		name string
		cfg  *config.Config
		want error
	}{
		{"no JWT secret", &config.Config{HTTP: config.HTTP{Port: "8080"}, Valkey: config.Valkey{URL: "valkey://localhost:6379/0"}, Mongo: config.Mongo{URL: _mongoURL}}, ErrNoJWTSecret},
		{"no Valkey URL", &config.Config{HTTP: config.HTTP{Port: "8080"}, JWT: config.JWT{Secret: "secret"}, Mongo: config.Mongo{URL: _mongoURL}}, ErrNoValkeyURL},
		{"no Mongo URL", &config.Config{HTTP: config.HTTP{Port: "8080"}, JWT: config.JWT{Secret: "secret"}, Valkey: config.Valkey{URL: "valkey://localhost:6379/0"}}, ErrNoMongoURL},
		{"a trusted proxy that is no address", &config.Config{
			HTTP: config.HTTP{Port: "8080", TrustedProxy: "not-an-address"}, JWT: config.JWT{Secret: "secret"},
			Valkey: config.Valkey{URL: "valkey://localhost:6379/0"}, Mongo: config.Mongo{URL: _mongoURL},
		}, errBadProxy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, Run(t.Context(), tt.cfg), tt.want)
		})
	}
}
