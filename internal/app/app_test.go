package app

import (
	"testing"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The worker starts without HTTP_PORT and JWT_SECRET, so config leaves them optional; the app
// refuses to start without either, before it opens anything.
func TestTheAppRefusesToStartWithoutItsSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		port    string
		secret  string
		want    []error
		notWant []error
	}{
		{name: "no HTTP port", secret: "s", want: []error{ErrNoHTTPPort}, notWant: []error{ErrNoJWTSecret}},
		{name: "no JWT secret", port: "8080", want: []error{ErrNoJWTSecret}, notWant: []error{ErrNoHTTPPort}},
		{name: "neither", want: []error{ErrNoHTTPPort, ErrNoJWTSecret}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Run(t.Context(), &config.Config{HTTP: config.HTTP{Port: tt.port}, JWT: config.JWT{Secret: tt.secret}})

			for _, want := range tt.want {
				require.ErrorIs(t, err, want)
			}

			for _, notWant := range tt.notWant {
				assert.NotErrorIs(t, err, notWant)
			}
		})
	}

	require.NoError(t, checkAppSettings(&config.Config{HTTP: config.HTTP{Port: "8080"}, JWT: config.JWT{Secret: "s"}}))
}
