package app_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/app"
	"github.com/m7medvision/go-clean-template/internal/entity"
	mongotask "github.com/m7medvision/go-clean-template/internal/entity/mongo/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parameterSpec is the part of the OpenAPI document the parameter tests read: every operation's
// parameters, keyed by its operation ID.
func parameterSpec(t *testing.T) map[string][]parameter {
	t.Helper()

	doc, err := app.OpenAPI("test")
	require.NoError(t, err)

	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string      `json:"operationId"`
			Parameters  []parameter `json:"parameters"`
		} `json:"paths"`
	}

	require.NoError(t, json.Unmarshal(doc, &spec))

	params := map[string][]parameter{}

	for _, ops := range spec.Paths {
		for _, op := range ops {
			params[op.OperationID] = op.Parameters
		}
	}

	return params
}

type parameter struct {
	Name   string `json:"name"`
	Schema struct {
		Default   float64  `json:"default"`
		Maximum   float64  `json:"maximum"`
		MaxLength *int     `json:"maxLength"`
		Enum      []string `json:"enum"`
	} `json:"schema"`
}

// The schema hints of every list's limit document the domain's paging rule (entity.Page).
func TestEveryLimitDocumentsTheDomainsPage(t *testing.T) {
	t.Parallel()

	limits := 0

	for op, params := range parameterSpec(t) {
		for _, p := range params {
			if p.Name != "limit" {
				continue
			}

			limits++

			assert.InDelta(t, entity.DefaultPageLimit, p.Schema.Default, 0, op)
			assert.InDelta(t, entity.MaxPageLimit, p.Schema.Maximum, 0, op)
		}
	}

	assert.Positive(t, limits)
}

// The query parameters' hints document the domain's rules: every status filter lists
// entity.TaskStatuses, and a passkey's name is at most entity.MaxPasskeyNameLen.
func TestQueryParameterHintsDocumentTheDomainsRules(t *testing.T) {
	t.Parallel()

	params := parameterSpec(t)
	filters := 0

	for op, ps := range params {
		for _, p := range ps {
			if p.Name == "status" {
				filters++

				assert.Equal(t, taskStatuses(), p.Schema.Enum, op)
			}
		}
	}

	assert.Equal(t, 2, filters, "list-tasks and list-all-tasks")

	var name *parameter

	for i, p := range params["finish-passkey-registration"] {
		if p.Name == "name" {
			name = &params["finish-passkey-registration"][i]
		}
	}

	require.NotNil(t, name, "finish-passkey-registration's name")
	require.NotNil(t, name.Schema.MaxLength)
	assert.Equal(t, entity.MaxPasskeyNameLen, *name.Schema.MaxLength)
}

func taskStatuses() []string {
	var names []string
	for _, s := range entity.TaskStatuses() {
		names = append(names, string(s))
	}

	return names
}

// The REST schema's length, pattern and enum hints document the domain's rules
// (entity.ValidateTaskText, entity.ValidateSignUp, entity.NewConversion, the passkey name,
// entity.TaskStatuses), which every transport enforces.
func TestBodyLengthHintsDocumentTheDomainsRules(t *testing.T) {
	t.Parallel()

	doc, err := app.OpenAPI("test")
	require.NoError(t, err)

	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					MinLength *int     `json:"minLength"`
					MaxLength *int     `json:"maxLength"`
					Pattern   string   `json:"pattern"`
					Enum      []string `json:"enum"`
					Minimum   *float64 `json:"minimum"`
					Maximum   *float64 `json:"maximum"`
					MaxItems  *int     `json:"maxItems"`
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

	for _, name := range []string{"Task", "TransitionTask"} {
		assert.Equal(t, taskStatuses(), schemas[name].Properties["status"].Enum, name)
	}

	priorities := make([]string, 0, len(mongotask.Priorities()))
	for _, p := range mongotask.Priorities() {
		priorities = append(priorities, string(p))
	}

	for _, name := range []string{"TaskAttributes", "UpdateTaskAttributes"} {
		attrs := schemas[name].Properties
		assert.Equal(t, priorities, attrs["priority"].Enum, name)
		assert.InDelta(t, mongotask.MinEstimate, *attrs["estimate"].Minimum, 0, name)
		assert.InDelta(t, mongotask.MaxEstimate, *attrs["estimate"].Maximum, 0, name)
		assert.Equal(t, mongotask.MaxLabels, *attrs["labels"].MaxItems, name)
	}
}
