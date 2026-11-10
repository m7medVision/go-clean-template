// Package flagstest builds a flags.Client with fixed values, for tests of the code that checks a flag.
package flagstest

import (
	"log/slog"
	"testing"

	"github.com/m7medvision/go-clean-template/pkg/flags"
	"github.com/open-feature/go-sdk/openfeature/memprovider"
)

// New answers values; a key it does not hold answers its caller's default.
func New(tb testing.TB, values map[string]bool) *flags.Client {
	tb.Helper()

	in := make(map[string]memprovider.InMemoryFlag, len(values))
	for k, v := range values {
		in[k] = memprovider.InMemoryFlag{
			Key: k, State: memprovider.Enabled, DefaultVariant: "v", Variants: map[string]any{"v": v},
		}
	}

	c, err := flags.NewWithProvider(tb.Context(), memprovider.NewInMemoryProvider(in), slog.New(slog.DiscardHandler))
	if err != nil {
		tb.Fatalf("flagstest.New: %v", err)
	}

	return c
}
