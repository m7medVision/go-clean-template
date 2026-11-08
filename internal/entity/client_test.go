package entity_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
)

func TestClientFromContext(t *testing.T) {
	t.Parallel()

	client := netip.MustParseAddr("203.0.113.7")

	tests := []struct {
		name   string
		ctx    context.Context
		want   netip.Addr
		wantOK bool
	}{
		{"the client it carries", entity.ContextWithClient(t.Context(), client), client, true},
		{"none when it carries none", t.Context(), netip.Addr{}, false},
		{"none when it carries an invalid address", entity.ContextWithClient(t.Context(), netip.Addr{}), netip.Addr{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := entity.ClientFromContext(tt.ctx)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
