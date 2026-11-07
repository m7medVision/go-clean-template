package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m7medvision/go-clean-template/pkg/valkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vk "github.com/valkey-io/valkey-go"
)

const scratchKeyTTL = time.Minute

func TestValkeySetsReadsAndDeletesAKey(t *testing.T) {
	valkeyURL := os.Getenv("VALKEY_URL")
	require.NotEmpty(t, valkeyURL, "VALKEY_URL is not set")

	ctx, cancel := context.WithTimeout(t.Context(), connectWait)
	defer cancel()

	client, err := valkey.New(ctx, valkeyURL)
	require.NoError(t, err)

	defer client.Close()

	key := "_test:" + uuid.NewString()
	value := uuid.NewString()

	err = client.Do(ctx, client.B().Set().Key(key).Value(value).Ex(scratchKeyTTL).Build()).Error()
	require.NoError(t, err)

	got, err := client.Do(ctx, client.B().Get().Key(key).Build()).ToString()
	require.NoError(t, err)
	assert.Equal(t, value, got)

	deleted, err := client.Do(ctx, client.B().Del().Key(key).Build()).AsInt64()
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	err = client.Do(ctx, client.B().Get().Key(key).Build()).Error()
	assert.True(t, vk.IsValkeyNil(err), "GET after DEL: %v", err)
}
