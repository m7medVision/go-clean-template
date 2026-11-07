package edit_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const taskSchema = `type Task { id: ID! }

type Query {
  task(id: ID!): Task!
}

type Mutation {
  deleteTask(id: ID!): Boolean!
}
`

const exchangeSchema = `"""
A conversion.
"""
type Conversion { rate: String! }

extend type Query {
  exchangeHistory(limit: Int! = 10): [Conversion!]!
}
`

func parseGraphQL(t *testing.T, src string) *edit.GraphQL {
	t.Helper()

	q, err := edit.ParseGraphQL("schema.graphqls", []byte(src))
	require.NoError(t, err)

	return q
}

func TestGraphQLListsTheTypesItDefines(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"Conversion"}, parseGraphQL(t, exchangeSchema).Types())
}

// Every file of a schema can add fields to a type, as an extension adds them to Query, so a field
// is named after its type too.
func TestGraphQLListsTheFieldsOfEveryTypeItDefinesOrExtends(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		src  string
		want []string
	}{
		"definitions":   {taskSchema, []string{"Task.id", "Query.task", "Mutation.deleteTask"}},
		"an extension":  {exchangeSchema, []string{"Conversion.rate", "Query.exchangeHistory"}},
		"in file order": {"extend type Query { a: Int }\n\ninput B { b: Int }\n\nenum C { X }\n", []string{"Query.a", "B.b"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, parseGraphQL(t, tc.src).Fields())
		})
	}
}

func TestParseGraphQLRejectsBrokenSource(t *testing.T) {
	t.Parallel()

	_, err := edit.ParseGraphQL("schema.graphqls", []byte("type Task {"))
	require.Error(t, err)
}
