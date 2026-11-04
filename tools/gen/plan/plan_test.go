package plan_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const busesSrc = `package app

func registerHandlers() {
	task.Register(cmds, qs)
	mail.Register(cmds)

	return nil
}

func subscribers() []repo.Subscriber {
	return []repo.Subscriber{task.NewProjector(pg)}
}
`

const schemaSrc = "type Query {\n  task: Int\n}\n"

// fixturePlan creates two files and edits two, one of them twice and once mid-line.
func fixturePlan() *plan.Plan {
	p := plan.New("new-domain widget")

	p.Create("internal/usecase/widget/register.go", []byte("package widget\n"))
	p.Create("internal/entity/widget.go", []byte("package entity\n"))
	p.Edit("internal/app/buses.go", []byte(busesSrc),
		edit.Insertion{Offset: bytes.Index([]byte(busesSrc), []byte("\n\treturn nil")), Text: "\twidget.Register(cmds, qs)\n"},
		edit.Insertion{Offset: bytes.Index([]byte(busesSrc), []byte("}\n}\n")), Text: ", widget.NewProjector(pg)"})
	p.Edit("internal/controller/graphql/schema/task.graphqls", []byte(schemaSrc),
		edit.Insertion{Offset: len("type Query {\n  task: Int\n"), Text: "  widget: Int\n"})

	return p
}

func TestChangesApplyTheInsertionsAndFormatGo(t *testing.T) {
	t.Parallel()

	changes, err := fixturePlan().Changes()
	require.NoError(t, err)

	got := map[string]string{}
	for _, c := range changes {
		got[c.Path] = string(c.New)
	}

	assert.Equal(t, `package app

func registerHandlers() {
	task.Register(cmds, qs)
	mail.Register(cmds)
	widget.Register(cmds, qs)

	return nil
}

func subscribers() []repo.Subscriber {
	return []repo.Subscriber{task.NewProjector(pg), widget.NewProjector(pg)}
}
`, got["internal/app/buses.go"])
	assert.Equal(t, "type Query {\n  task: Int\n  widget: Int\n}\n", got["internal/controller/graphql/schema/task.graphqls"])
	assert.Equal(t, "package widget\n", got["internal/usecase/widget/register.go"])

	again, err := fixturePlan().Changes()
	require.NoError(t, err)
	assert.Equal(t, changes, again)
}

func TestChangesRejectAnEditThatBreaksAGoFile(t *testing.T) {
	t.Parallel()

	p := plan.New("x")
	p.Edit("a.go", []byte("package a\n"), edit.Insertion{Offset: 0, Text: "}"})

	_, err := p.Changes()
	require.ErrorIs(t, err, plan.ErrInvalid)
}

func TestListsAreSortedByPath(t *testing.T) {
	t.Parallel()

	p := fixturePlan()

	assert.Equal(t, []string{"internal/entity/widget.go", "internal/usecase/widget/register.go"}, p.Created())
	assert.Equal(t, []string{"internal/app/buses.go", "internal/controller/graphql/schema/task.graphqls"}, p.Edited())
}

func TestDryRunMatchesItsGolden(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	require.NoError(t, fixturePlan().DryRun(&out))

	golden(t, filepath.Join("testdata", "dryrun.golden"), out.Bytes())
}

func TestUndoRestoresEditsAndRemovesWhatWasAdded(t *testing.T) {
	t.Parallel()

	got := plan.Undo(
		[]string{"internal/app/buses.go", "internal/entity/event.go"},
		[]string{"docs/openapi/v1.json", "internal/repo/repomock"},
		[]string{"internal/usecase/widget", "internal/entity/widget.go"},
	)

	assert.Equal(t, "git checkout -- internal/app/buses.go internal/entity/event.go docs/openapi/v1.json internal/repo/repomock && "+
		"git clean -fd -- internal/usecase/widget internal/entity/widget.go docs/openapi/v1.json internal/repo/repomock", got)
}

// golden compares got with the file at path; UPDATE_GOLDEN=1 rewrites the file instead.
func golden(t *testing.T, path string, got []byte) {
	t.Helper()

	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.WriteFile(path, got, 0o600))
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}
