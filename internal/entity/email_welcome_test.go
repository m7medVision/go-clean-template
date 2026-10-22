package entity_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeEmail_RoundTrips(t *testing.T) {
	t.Parallel()

	sent := entity.WelcomeEmail{To: "john@example.com", Username: "johndoe", LoginURL: "https://example.com/login"}

	body, err := entity.EncodeEmail(sent)
	require.NoError(t, err)

	got, err := entity.DecodeEmail(body)
	require.NoError(t, err)

	assert.Equal(t, sent, got)
	assert.Equal(t, "john@example.com", got.Recipient())
}
