package entity_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasskeyID_TextRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   entity.PasskeyID
		text string
	}{
		{name: "url-safe alphabet, no padding", id: entity.PasskeyID{0xfb, 0xff, 0xbe}, text: "-_--"},
		{name: "length not a multiple of three", id: entity.PasskeyID{0x01, 0x02}, text: "AQI"},
		{name: "empty", id: entity.PasskeyID{}, text: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			text, err := tt.id.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, tt.text, string(text))
			assert.Equal(t, tt.text, tt.id.String())

			var got entity.PasskeyID

			require.NoError(t, got.UnmarshalText(text))
			assert.Equal(t, []byte(tt.id), []byte(got))
		})
	}
}

func TestPasskeyID_UnmarshalTextRejectsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
	}{
		{name: "padded", text: "AQI="},
		{name: "standard alphabet", text: "+/+/"},
		{name: "not base64", text: "!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got entity.PasskeyID

			require.ErrorIs(t, got.UnmarshalText([]byte(tt.text)), entity.ErrPasskeyNotFound)
		})
	}
}

func TestPasskey_JSONHidesCredentialAndUser(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	passkey := entity.Passkey{
		ID:         entity.PasskeyID{0x01, 0x02},
		UserID:     "550e8400-e29b-41d4-a716-446655440000",
		Name:       "Laptop",
		Credential: json.RawMessage(`{"secret":true}`),
		CreatedAt:  created,
	}

	body, err := json.Marshal(passkey)
	require.NoError(t, err)

	assert.JSONEq(t,
		`{"id":"AQI","name":"Laptop","created_at":"2026-01-01T00:00:00Z","last_used_at":null}`,
		string(body))
}
