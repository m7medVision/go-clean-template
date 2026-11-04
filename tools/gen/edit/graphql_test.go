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
  "Deletes it. A } in a description is not the end."
  deleteTask(id: ID!): Boolean!
  # nor is one } in a comment
}
`

// exchangeSchema extends Query and has text before it with runes wider than a byte.
const exchangeSchema = `"""
A conversion — rates are decimal strings { braces } included.
"""
type Conversion { rate: String! }

extend type Query {
  "One page, newest first. limit is 1–100."
  exchangeHistory(limit: Int! = 10): [Conversion!]!
}
`

func parseGraphQL(t *testing.T, src string) *edit.GraphQL {
	t.Helper()

	q, err := edit.ParseGraphQL("schema.graphqls", []byte(src))
	require.NoError(t, err)

	return q
}

func TestInTypeAddsAFieldToTheRootType(t *testing.T) {
	t.Parallel()

	ins, err := parseGraphQL(t, taskSchema).InType("Mutation", "archiveTask(id: ID!): ID!")
	require.NoError(t, err)

	got := edit.Apply([]byte(taskSchema), []edit.Insertion{ins})

	assert.Equal(t, `type Task { id: ID! }

type Query {
  task(id: ID!): Task!
}

type Mutation {
  "Deletes it. A } in a description is not the end."
  deleteTask(id: ID!): Boolean!
  # nor is one } in a comment
  archiveTask(id: ID!): ID!
}
`, string(got))

	_, err = edit.ParseGraphQL("schema.graphqls", got)
	require.NoError(t, err)
}

func TestInTypeAddsAFieldToAnExtension(t *testing.T) {
	t.Parallel()

	ins, err := parseGraphQL(t, exchangeSchema).InType("Query", "countConversions: Int!")
	require.NoError(t, err)

	assert.Equal(t, `"""
A conversion — rates are decimal strings { braces } included.
"""
type Conversion { rate: String! }

extend type Query {
  "One page, newest first. limit is 1–100."
  exchangeHistory(limit: Int! = 10): [Conversion!]!
  countConversions: Int!
}
`, string(edit.Apply([]byte(exchangeSchema), []edit.Insertion{ins})))
}

func TestInTypeNeedsTheTypeOnce(t *testing.T) {
	t.Parallel()

	_, err := parseGraphQL(t, exchangeSchema).InType("Mutation", "x: Int")
	require.ErrorIs(t, err, edit.ErrAnchor, "missing")

	twice := taskSchema + "\nextend type Query {\n  other: Int\n}\n"
	_, err = parseGraphQL(t, twice).InType("Query", "x: Int")
	require.ErrorIs(t, err, edit.ErrAnchor, "twice")
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

func TestAtEndAppendsADefinition(t *testing.T) {
	t.Parallel()

	q := parseGraphQL(t, taskSchema)
	got := edit.Apply([]byte(taskSchema), []edit.Insertion{q.AtEnd("type CountTasksResult { todo: Boolean }")})

	_, err := edit.ParseGraphQL("schema.graphqls", got)
	require.NoError(t, err)
	assert.Contains(t, string(got), "}\n\ntype CountTasksResult { todo: Boolean }\n")
}
