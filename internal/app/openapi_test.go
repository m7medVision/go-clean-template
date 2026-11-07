package app_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/app"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The schema hints of every list's limit document the domain's paging rule (entity.Page).
func TestEveryLimitDocumentsTheDomainsPage(t *testing.T) {
	t.Parallel()

	doc, err := app.OpenAPI("test")
	require.NoError(t, err)

	var spec struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name   string `json:"name"`
				Schema struct {
					Default float64 `json:"default"`
					Maximum float64 `json:"maximum"`
				} `json:"schema"`
			} `json:"parameters"`
		} `json:"paths"`
	}

	require.NoError(t, json.Unmarshal(doc, &spec))

	limits := 0

	for path, ops := range spec.Paths {
		for method, op := range ops {
			for _, p := range op.Parameters {
				if p.Name != "limit" {
					continue
				}

				limits++

				assert.InDelta(t, entity.DefaultPageLimit, p.Schema.Default, 0, method+" "+path)
				assert.InDelta(t, entity.MaxPageLimit, p.Schema.Maximum, 0, method+" "+path)
			}
		}
	}

	assert.Positive(t, limits)
}

// The REST schema's length and pattern hints document the domain's rules (entity.ValidateTaskText,
// entity.ValidateSignUp, entity.NewConversion, the passkey name), which every transport enforces.
func TestBodyLengthHintsDocumentTheDomainsRules(t *testing.T) {
	t.Parallel()

	doc, err := app.OpenAPI("test")
	require.NoError(t, err)

	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					MinLength *int   `json:"minLength"`
					MaxLength *int   `json:"maxLength"`
					Pattern   string `json:"pattern"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}

	require.NoError(t, json.Unmarshal(doc, &spec))

	schemas := spec.Components.Schemas

	for _, name := range []string{"CreateTask", "UpdateTask"} {
		assert.Equal(t, 1, *schemas[name].Properties["title"].MinLength, name)
		assert.Equal(t, entity.MaxTaskTitleLen, *schemas[name].Properties["title"].MaxLength, name)
		assert.Equal(t, entity.MaxTaskDescriptionLen, *schemas[name].Properties["description"].MaxLength, name)
	}

	assert.Equal(t, entity.MinUsernameLen, *schemas["Register"].Properties["username"].MinLength)
	assert.Equal(t, entity.MaxUsernameLen, *schemas["Register"].Properties["username"].MaxLength)
	assert.Equal(t, entity.MinPasswordLen, *schemas["Register"].Properties["password"].MinLength)
	assert.Equal(t, entity.MaxPasskeyNameLen, *schemas["RenamePasskey"].Properties["name"].MaxLength)
	assert.Equal(t, fmt.Sprintf(`^[0-9]{1,%d}(\.[0-9]{1,%d})?$`, entity.MaxAmountDigits, entity.MaxAmountScale),
		schemas["Convert"].Properties["amount"].Pattern)
}
