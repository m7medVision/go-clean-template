package app

import (
	"log/slog"
	"testing"

	"github.com/m7medvision/go-clean-template/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithoutADSNTheLoggerIsNotWrapped(t *testing.T) {
	t.Parallel()

	l := slog.New(slog.DiscardHandler)

	got, flush, err := errorTracking(l, &config.Config{}, "app")
	require.NoError(t, err)
	assert.Same(t, l, got)
	flush()
}
