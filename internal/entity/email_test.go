package entity_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/evrone/go-clean-template/internal/entity"
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

func TestEncodeEmail_NamesTheTemplate(t *testing.T) {
	t.Parallel()

	body, err := entity.EncodeEmail(entity.WelcomeEmail{To: "john@example.com"})
	require.NoError(t, err)

	var envelope struct {
		Template string `json:"template"`
	}

	require.NoError(t, json.Unmarshal(body, &envelope))
	assert.Equal(t, "welcome", envelope.Template)
}

func TestDecodeEmail_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		wantUnknown bool
	}{
		{"unknown template", `{"template":"nope","data":{}}`, true},
		{"missing template", `{"data":{}}`, true},
		{"body is not JSON", `not json`, false},
		{"data does not match the template", `{"template":"welcome","data":"a string"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := entity.DecodeEmail(json.RawMessage(tt.body))

			require.Error(t, err)
			assert.Equal(t, tt.wantUnknown, errors.Is(err, entity.ErrUnknownEmail))
		})
	}
}
