package graphql_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/controller/graphql/generated"
	"github.com/m7medvision/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The schema's defaults and enums document the domain's rules: every list's limit defaults to
// entity.DefaultPageLimit, and TaskStatus has a value for each of entity.TaskStatuses, named after
// it.
func TestTheSchemaDocumentsTheDomainsRules(t *testing.T) {
	t.Parallel()

	schema := generated.NewExecutableSchema(generated.Config{}).Schema()

	limits := 0

	for _, typ := range schema.Types {
		for _, field := range typ.Fields {
			for _, arg := range field.Arguments {
				if arg.Name != "limit" {
					continue
				}

				limits++

				require.NotNil(t, arg.DefaultValue, typ.Name+"."+field.Name)
				assert.Equal(t, strconv.Itoa(entity.DefaultPageLimit), arg.DefaultValue.Raw, typ.Name+"."+field.Name)
			}
		}
	}

	assert.Positive(t, limits)

	status := schema.Types["TaskStatus"]
	require.NotNil(t, status)

	var values []entity.TaskStatus
	for _, v := range status.EnumValues {
		values = append(values, entity.TaskStatus(strings.ToLower(v.Name)))
	}

	assert.Equal(t, entity.TaskStatuses(), values)
}
