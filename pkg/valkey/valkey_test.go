package valkey_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/pkg/valkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _timeout = 5 * time.Second

func TestNew_Fails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{name: "not a URL", url: "://127.0.0.1:6379"},
		{name: "unknown scheme", url: "http://127.0.0.1:6379/0"},
		{name: "nothing listening", url: "valkey://" + closedAddr(t) + "/0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), _timeout)
			defer cancel()

			client, err := valkey.New(ctx, tt.url)
			require.Error(t, err)
			assert.Nil(t, client)
		})
	}
}

func closedAddr(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	return addr
}
