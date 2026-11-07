package edit_test

import (
	"go/format"
	"go/token"
	"testing"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// applyGo applies ins to src and formats the result, the way a plan does with an edited Go file.
func applyGo(t *testing.T, src string, ins ...edit.Insertion) string {
	t.Helper()

	out, err := format.Source(edit.Apply([]byte(src), ins))
	require.NoError(t, err)

	return string(out)
}

func parseGo(t *testing.T, src string) *edit.Go {
	t.Helper()

	g, err := edit.ParseGo("x.go", []byte(src))
	require.NoError(t, err)

	return g
}

const errorsSrc = `package entity

import "errors"

var (
	ErrTaskNotFound  = errors.New("task not found")
	ErrTaskForbidden = errors.New("task does not belong to user")
	// ErrReadModelBehind is a write that committed late.
	ErrReadModelBehind = errors.New("write saved but not yet readable")
)

// After the block.
func f() {}
`

func TestBeforeBlockEndAppendsToTheVarBlock(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, errorsSrc).BeforeBlockEnd(token.VAR,
		"ErrWidgetNotFound  = errors.New(\"widget not found\")\nErrWidgetForbidden = errors.New(\"widget does not belong to user\")")
	require.NoError(t, err)

	assert.Equal(t, `package entity

import "errors"

var (
	ErrTaskNotFound  = errors.New("task not found")
	ErrTaskForbidden = errors.New("task does not belong to user")
	// ErrReadModelBehind is a write that committed late.
	ErrReadModelBehind = errors.New("write saved but not yet readable")
	ErrWidgetNotFound  = errors.New("widget not found")
	ErrWidgetForbidden = errors.New("widget does not belong to user")
)

// After the block.
func f() {}
`, applyGo(t, errorsSrc, ins))
}

const contractsSrc = `package repo

//go:generate mockgen -source ./contracts.go -package repomock

type (
	// TaskReadModel reads the tasks table.
	TaskReadModel interface {
		Get(id string) error
	}
)
`

func TestBeforeBlockEndAppendsToTheTypeBlock(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, contractsSrc).BeforeBlockEnd(token.TYPE,
		"\n// WidgetReadModel reads the widgets table.\nWidgetReadModel interface {\n\tGet(id string) error\n}")
	require.NoError(t, err)

	assert.Equal(t, `package repo

//go:generate mockgen -source ./contracts.go -package repomock

type (
	// TaskReadModel reads the tasks table.
	TaskReadModel interface {
		Get(id string) error
	}

	// WidgetReadModel reads the widgets table.
	WidgetReadModel interface {
		Get(id string) error
	}
)
`, applyGo(t, contractsSrc, ins))
}

func TestBeforeBlockEndNeedsExactlyOneBlock(t *testing.T) {
	t.Parallel()

	for name, src := range map[string]string{
		"none":         "package p\n\nvar x = 1\n",
		"two":          "package p\n\nvar (\n\ta = 1\n)\n\nvar (\n\tb = 2\n)\n",
		"another kind": "package p\n\ntype (\n\tT int\n)\n",
	} {
		_, err := parseGo(t, src).BeforeBlockEnd(token.VAR, "c = 3")
		require.ErrorIs(t, err, edit.ErrAnchor, name)
	}
}

const importsSrc = `package app

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/repo"
	persistTaskRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/task"
)
`

func TestImportGoesInPathOrderInTheNonStandardGroup(t *testing.T) {
	t.Parallel()

	g := parseGo(t, importsSrc)

	aliased, err := g.Import("github.com/m7medvision/go-clean-template/internal/repo/persistent/invoice", "persistInvoiceRepo")
	require.NoError(t, err)

	plain, err := g.Import("github.com/m7medvision/go-clean-template/internal/usecase/invoice", "")
	require.NoError(t, err)

	assert.Equal(t, `package app

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/repo"
	persistInvoiceRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/invoice"
	persistTaskRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/invoice"
	"github.com/m7medvision/go-clean-template/internal/usecase/task"
)
`, applyGo(t, importsSrc, append(aliased, plain...)...))
}

func TestImportAddsNothingWhenTheFileHasIt(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, importsSrc).Import("fmt", "")
	require.NoError(t, err)
	assert.Empty(t, ins)
}

func TestImportRefusesTheSamePathUnderAnotherAlias(t *testing.T) {
	t.Parallel()

	_, err := parseGo(t, importsSrc).Import("github.com/m7medvision/go-clean-template/internal/usecase/task", "taskuc")
	require.ErrorIs(t, err, edit.ErrAnchor)
}

// A domain called context or fmt would be imported under a name the file already binds, and the
// file would no longer compile.
func TestImportRefusesANameAnotherImportBinds(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ path, alias, bound string }{
		{"github.com/m7medvision/go-clean-template/internal/usecase/fmt", "", "fmt"},
		{"github.com/m7medvision/go-clean-template/internal/usecase/context", "", "context"},
		{"github.com/m7medvision/go-clean-template/internal/repo/persistent/thing", "persistTaskRepo", "persistTaskRepo"},
		{"github.com/m7medvision/go-clean-template/internal/repo/task/v2", "", "task"},
	} {
		_, err := parseGo(t, importsSrc).Import(tc.path, tc.alias)
		require.ErrorIs(t, err, edit.ErrAnchor, tc.path)
		assert.Contains(t, err.Error(), tc.bound+" is imported already", tc.path)
	}
}

// A test file imports the package under test under a name of its own choosing: task's gRPC test
// as task, a generated domain's as grpcwidget. What a test appended to it calls must use that name.
func TestImportNameIsTheNameTheFileBindsAPathTo(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		path, alias, want string
		inserts           bool
	}{
		"imported plainly":     {"github.com/m7medvision/go-clean-template/internal/usecase/task", "uc", "task", false},
		"imported as an alias": {"github.com/m7medvision/go-clean-template/internal/repo/persistent/task", "", "persistTaskRepo", false},
		"not imported":         {"github.com/m7medvision/go-clean-template/docs/proto/v1", "pb", "pb", true},
		"not imported, plain":  {"github.com/m7medvision/go-clean-template/internal/entity", "", "entity", true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, ins, err := parseGo(t, importsSrc).ImportName(tc.path, tc.alias)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.inserts, len(ins) == 1, "imports it only when the file does not")
		})
	}

	_, _, err := parseGo(t, importsSrc).ImportName("github.com/m7medvision/go-clean-template/internal/usecase/fmt", "")
	require.ErrorIs(t, err, edit.ErrAnchor, "a name another import binds")
}

func TestImportStartsTheNonStandardGroupAfterTheStandardOne(t *testing.T) {
	t.Parallel()

	const src = "package p\n\nimport (\n\t\"context\"\n)\n"

	ins, err := parseGo(t, src).Import("github.com/x/y", "")
	require.NoError(t, err)

	assert.Equal(t, "package p\n\nimport (\n\t\"context\"\n\n\t\"github.com/x/y\"\n)\n", applyGo(t, src, ins...))
}

// A standard path goes into the standard group, in path order, the way gci sorts it; a file with
// none starts that group ahead of the others.
func TestImportPutsAStandardPathInTheStandardGroup(t *testing.T) {
	t.Parallel()

	const nonStandardOnly = "package p\n\nimport (\n\t\"github.com/x/y\"\n)\n"

	for name, tc := range map[string]struct{ src, path, want string }{
		"among the standard ones": {
			src: importsSrc, path: "errors",
			want: "import (\n\t\"context\"\n\t\"errors\"\n\t\"fmt\"\n\n\t\"github.com/m7medvision/",
		},
		"after the last standard one": {
			src: importsSrc, path: "net/http",
			want: "\t\"fmt\"\n\t\"net/http\"\n\n\t\"github.com/m7medvision/",
		},
		"starting the standard group": {
			src: nonStandardOnly, path: "context",
			want: "import (\n\t\"context\"\n\n\t\"github.com/x/y\"\n)\n",
		},
		"in an empty block": {
			src: "package p\n\nimport (\n)\n", path: "context",
			want: "import (\n\t\"context\"\n)\n",
		},
	} {
		ins, err := parseGo(t, tc.src).Import(tc.path, "")
		require.NoError(t, err, name)
		assert.Contains(t, applyGo(t, tc.src, ins...), tc.want, name)
	}
}

func TestConstFindsOneTopLevelConstantByPattern(t *testing.T) {
	t.Parallel()

	g := parseGo(t, "package p\n\nconst _tasksTag = \"tasks\"\n\nconst (\n\t_a, _bTag = 1, 2\n\tc = 3\n)\n\nvar _vTag = 4\n")

	got, err := g.Const("_task*Tag")
	require.NoError(t, err)
	assert.Equal(t, "_tasksTag", got)

	_, err = g.Const("_*Tag")
	require.ErrorIs(t, err, edit.ErrAnchor, "_tasksTag and _bTag")
	assert.Contains(t, err.Error(), "a constant _*Tag")

	_, err = g.Const("_v*")
	require.ErrorIs(t, err, edit.ErrAnchor, "a var is no constant")
}

func TestFieldOfTypeFindsOneFieldOfAStructByItsType(t *testing.T) {
	t.Parallel()

	g := parseGo(t, `package p

type Deps struct {
	Store         repo.EventStore
	History       repo.ExchangeReadModel
	Rates, Backup repo.ExchangeWebAPI
	Now           func() time.Time
}

type Embeds struct {
	repo.TaskReadModel
}
`)

	for pattern, want := range map[string][2]string{
		"repo.*ReadModel": {"History", "repo.ExchangeReadModel"},
		"repo.EventStore": {"Store", "repo.EventStore"},
	} {
		field, typ, err := g.FieldOfType("Deps", pattern)
		require.NoError(t, err, pattern)
		assert.Equal(t, want, [2]string{field, typ}, pattern)
	}

	for pattern, why := range map[string]string{
		"repo.*WebAPI":  "two fields share the type",
		"repo.Missing":  "no field has the type",
		"time.Duration": "no field has the type",
	} {
		_, _, err := g.FieldOfType("Deps", pattern)
		require.ErrorIs(t, err, edit.ErrAnchor, why)
		assert.Contains(t, err.Error(), "a field of Deps of type "+pattern, why)
	}

	field, typ, err := g.FieldOfType("Embeds", "repo.*ReadModel")
	require.NoError(t, err)
	assert.Equal(t, [2]string{"TaskReadModel", "repo.TaskReadModel"}, [2]string{field, typ}, "named after its type")

	_, _, err = g.FieldOfType("Missing", "repo.EventStore")
	require.ErrorIs(t, err, edit.ErrAnchor)
	assert.Contains(t, err.Error(), "a struct type Missing")
}

func TestImportNeedsAnImportBlock(t *testing.T) {
	t.Parallel()

	_, err := parseGo(t, "package entity\n\nimport \"errors\"\n").Import("fmt", "")
	require.ErrorIs(t, err, edit.ErrAnchor)
}

func TestFuncFindsOneTopLevelFunctionOrMethod(t *testing.T) {
	t.Parallel()

	g := parseGo(t, "package p\n\nfunc a() {}\n\nfunc (r *c) problem() {}\n\nfunc (r *c) b() {}\n\nfunc (s s) b() {}\n")

	require.NoError(t, g.RequireFunc("a"))
	require.NoError(t, g.RequireFunc("problem"))
	require.ErrorIs(t, g.RequireFunc("missing"), edit.ErrAnchor)
	require.ErrorIs(t, g.RequireFunc("b"), edit.ErrAnchor)
}

func TestApplyKeepsTheOrderOfInsertionsAtOneOffset(t *testing.T) {
	t.Parallel()

	got := edit.Apply([]byte("ad"), []edit.Insertion{{Offset: 1, Text: "b"}, {Offset: 1, Text: "c"}, {Offset: 0, Text: ">"}})

	assert.Equal(t, ">abcd", string(got))
}
