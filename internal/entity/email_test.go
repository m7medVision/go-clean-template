package entity_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailTypes_EveryTypeRoundTripsAndHasAUniqueTemplate(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}

	for _, email := range entity.EmailTypes() {
		assert.False(t, seen[email.Template()], "template %q registered twice", email.Template())
		seen[email.Template()] = true

		body, err := entity.EncodeEmail(email)
		require.NoError(t, err)

		got, err := entity.DecodeEmail(body)
		require.NoError(t, err)
		assert.Equal(t, email, got)
	}
}

// noteEmail is a test-only email, so these tests do not depend on a real email type.
type noteEmail struct {
	To string `json:"to"`
}

func (noteEmail) Template() string    { return "note" }
func (e noteEmail) Recipient() string { return e.To }

func TestEncodeEmail_NamesTheTemplate(t *testing.T) {
	t.Parallel()

	body, err := entity.EncodeEmail(noteEmail{To: "john@example.com"})
	require.NoError(t, err)

	var envelope struct {
		Template string          `json:"template"`
		Data     json.RawMessage `json:"data"`
	}

	require.NoError(t, json.Unmarshal(body, &envelope))
	assert.Equal(t, "note", envelope.Template)
	assert.JSONEq(t, `{"to":"john@example.com"}`, string(envelope.Data))
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
		{"data does not match the template", `{"template":"welcome","data":"a string"}`, false}, // scaffold:welcome
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
