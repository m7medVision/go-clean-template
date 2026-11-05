package edit_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const gqlgenYAML = `schema:
  - schema/*.graphqls

# No resolver section.
omit_getters: true

models:
  Task:
    model: example.com/entity.Task
  TaskStatus:
    model: example.com/entity.TaskStatus
    enum_values:
      TODO:
        value: example.com/entity.TaskStatusTodo
`

const widgetModel = "model: example.com/entity.Widget"

func parseYAML(t *testing.T, src string) *edit.YAML {
	t.Helper()

	y, err := edit.ParseYAML("gqlgen.yml", []byte(src))
	require.NoError(t, err)

	return y
}

func TestInMapAddsAnEntryAfterTheLastOne(t *testing.T) {
	t.Parallel()

	ins, err := parseYAML(t, gqlgenYAML).InMap("models", "Widget", widgetModel)
	require.NoError(t, err)

	got := edit.Apply([]byte(gqlgenYAML), []edit.Insertion{ins})

	assert.Equal(t, gqlgenYAML+"  Widget:\n    model: example.com/entity.Widget\n", string(got))

	_, err = edit.ParseYAML("gqlgen.yml", got)
	require.NoError(t, err)
}

func TestInMapStopsAtTheNextTopLevelKey(t *testing.T) {
	t.Parallel()

	const src = "models:\n  Task:\n    model: x.Task\n\n# Then the rest.\nautobind: []\n"

	ins, err := parseYAML(t, src).InMap("models", "Widget", "model: x.Widget")
	require.NoError(t, err)

	assert.Equal(t, "models:\n  Task:\n    model: x.Task\n  Widget:\n    model: x.Widget\n\n# Then the rest.\nautobind: []\n",
		string(edit.Apply([]byte(src), []edit.Insertion{ins})))
}

func TestInMapRefusesWhatItCannotPlace(t *testing.T) {
	t.Parallel()

	for name, src := range map[string]string{
		"no models":     "schema:\n  - a\n",
		"flow mapping":  "models: {Task: {model: x.Task}}\n",
		"empty models":  "models:\n",
		"entry present": gqlgenYAML + "  Widget:\n    model: x.Widget\n",
	} {
		_, err := parseYAML(t, src).InMap("models", "Widget", widgetModel)
		require.ErrorIs(t, err, edit.ErrAnchor, name)
	}
}
