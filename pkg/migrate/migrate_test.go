package migrate_test

import (
	"log/slog"
	"testing"
	"testing/fstest"

	"github.com/m7medvision/go-clean-template/pkg/migrate"
	"github.com/stretchr/testify/require"
)

func TestOpenFails(t *testing.T) {
	t.Parallel()

	for name, url := range map[string]string{
		"an unparsable URL":             "postgres://%zz",
		"an address nothing listens on": "postgres://127.0.0.1:1/db?connect_timeout=1",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := migrate.Open(t.Context(), url, fstest.MapFS{}, slog.New(slog.DiscardHandler))
			require.Error(t, err)
		})
	}
}
