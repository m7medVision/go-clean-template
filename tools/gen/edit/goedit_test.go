package edit_test

import (
	"go/format"
	"go/token"
	"strings"
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

const aliasesSrc = `package graphql

// The aliases.
type (
	taskQuery    = task.Query
	taskMutation = task.Mutation
)

type query struct{}
`

func TestBeforeBlockEndAppendsToTheTypeBlock(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, aliasesSrc).BeforeBlockEnd(token.TYPE, "widgetQuery = widget.Query\nwidgetMutation = widget.Mutation")
	require.NoError(t, err)

	assert.Equal(t, `package graphql

// The aliases.
type (
	taskQuery      = task.Query
	taskMutation   = task.Mutation
	widgetQuery    = widget.Query
	widgetMutation = widget.Mutation
)

type query struct{}
`, applyGo(t, aliasesSrc, ins))
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

	queries, err := g.Import("github.com/m7medvision/go-clean-template/internal/repo/persistent/queries")
	require.NoError(t, err)

	invoice, err := g.Import("github.com/m7medvision/go-clean-template/internal/usecase/invoice")
	require.NoError(t, err)

	assert.Equal(t, `package app

import (
	"context"
	"fmt"

	"github.com/m7medvision/go-clean-template/internal/repo"
	"github.com/m7medvision/go-clean-template/internal/repo/persistent/queries"
	persistTaskRepo "github.com/m7medvision/go-clean-template/internal/repo/persistent/task"
	"github.com/m7medvision/go-clean-template/internal/usecase/invoice"
	"github.com/m7medvision/go-clean-template/internal/usecase/task"
)
`, applyGo(t, importsSrc, append(queries, invoice...)...))
}

func TestImportAddsNothingWhenTheFileHasIt(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, importsSrc).Import("fmt")
	require.NoError(t, err)
	assert.Empty(t, ins)
}

func TestImportRefusesAPathTheFileImportsUnderAnAlias(t *testing.T) {
	t.Parallel()

	_, err := parseGo(t, importsSrc).Import("github.com/m7medvision/go-clean-template/internal/repo/persistent/task")
	require.ErrorIs(t, err, edit.ErrAnchor)
}

// A domain called context or fmt would be imported under a name the file already binds, and the
// file would no longer compile.
func TestImportRefusesANameAnotherImportBinds(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ path, bound string }{
		{"github.com/m7medvision/go-clean-template/internal/usecase/fmt", "fmt"},
		{"github.com/m7medvision/go-clean-template/internal/usecase/context", "context"},
		{"github.com/m7medvision/go-clean-template/internal/repo/task/v2", "task"},
	} {
		_, err := parseGo(t, importsSrc).Import(tc.path)
		require.ErrorIs(t, err, edit.ErrAnchor, tc.path)
		assert.Contains(t, err.Error(), tc.bound+" is imported already", tc.path)
	}
}

func TestImportStartsTheNonStandardGroupAfterTheStandardOne(t *testing.T) {
	t.Parallel()

	const src = "package p\n\nimport (\n\t\"context\"\n)\n"

	ins, err := parseGo(t, src).Import("github.com/x/y")
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
		ins, err := parseGo(t, tc.src).Import(tc.path)
		require.NoError(t, err, name)
		assert.Contains(t, applyGo(t, tc.src, ins...), tc.want, name)
	}
}

func TestImportNeedsAnImportBlock(t *testing.T) {
	t.Parallel()

	_, err := parseGo(t, "package entity\n\nimport \"errors\"\n").Import("fmt")
	require.ErrorIs(t, err, edit.ErrAnchor)
}

const eventTypesSrc = `package entity

func EventTypes() []Event {
	return slices.Concat(
		taskEvents(),
		userEvents(), // the users
	)
}
`

func TestAppendArgAddsAnArgumentToAMultiLineCall(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, eventTypesSrc).AppendArg("EventTypes", "slices.Concat", "widgetEvents()")
	require.NoError(t, err)

	assert.Equal(t, `package entity

func EventTypes() []Event {
	return slices.Concat(
		taskEvents(),
		userEvents(), // the users
		widgetEvents(),
	)
}
`, applyGo(t, eventTypesSrc, ins))
}

func TestAppendArgAddsAnArgumentToAOneLineCall(t *testing.T) {
	t.Parallel()

	const src = "package p\n\nfunc f() {\n\tg(a, b)\n}\n"

	ins, err := parseGo(t, src).AppendArg("f", "g", "c")
	require.NoError(t, err)

	assert.Equal(t, "package p\n\nfunc f() {\n\tg(a, b, c)\n}\n", applyGo(t, src, ins))
}

func TestAppendArgNeedsExactlyOneCall(t *testing.T) {
	t.Parallel()

	g := parseGo(t, "package p\n\nfunc f() {\n\tg(a)\n\tg(b)\n}\n\nfunc h() {}\n")

	for name, args := range map[string][2]string{
		"no such func": {"missing", "g"},
		"no such call": {"h", "g"},
		"two calls":    {"f", "g"},
	} {
		_, err := g.AppendArg(args[0], args[1], "c")
		require.ErrorIs(t, err, edit.ErrAnchor, name)
	}
}

const registerSrc = `package app

func registerHandlers(cmds *cqrs.Commands, qs *cqrs.Queries) error {
	signIn, err := newSSO(cfg)
	if err != nil {
		return err
	}

	task.Register(cmds, qs, task.Deps{Store: store})
	exchange.Register(cmds, qs, exchange.Deps{
		Store: store,
	})
	mail.Register(cmds, mailDeps)

	return nil
}
`

func TestAfterLastStmtGoesAfterTheLastMatchingStatement(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, registerSrc).AfterLastStmt("registerHandlers", edit.Call("*.Register"),
		"widget.Register(cmds, qs, widget.Deps{\n\tStore: store,\n})")
	require.NoError(t, err)

	assert.Equal(t, `package app

func registerHandlers(cmds *cqrs.Commands, qs *cqrs.Queries) error {
	signIn, err := newSSO(cfg)
	if err != nil {
		return err
	}

	task.Register(cmds, qs, task.Deps{Store: store})
	exchange.Register(cmds, qs, exchange.Deps{
		Store: store,
	})
	mail.Register(cmds, mailDeps)
	widget.Register(cmds, qs, widget.Deps{
		Store: store,
	})

	return nil
}
`, applyGo(t, registerSrc, ins))
}

func TestAfterLastStmtMatchesAnAssignedCall(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, registerSrc).AfterLastStmt("registerHandlers", edit.Call("newSSO"), "other := 1")
	require.NoError(t, err)
	assert.Contains(t, applyGo(t, registerSrc, ins), "signIn, err := newSSO(cfg)\n\tother := 1\n")

	_, err = parseGo(t, registerSrc).AfterLastStmt("registerHandlers", edit.Call("*.Deregister"), "x")
	require.ErrorIs(t, err, edit.ErrAnchor)
}

// The matching statements are the place only when they are one run: split in two, the new one
// could go after either, so the place is found twice.
func TestAfterLastStmtRefusesMatchesInTwoRuns(t *testing.T) {
	t.Parallel()

	split := strings.Replace(registerSrc, "\tmail.Register(cmds, mailDeps)\n", "\tlog()\n\tmail.Register(cmds, mailDeps)\n", 1)

	_, err := parseGo(t, split).AfterLastStmt("registerHandlers", edit.Call("*.Register"), "x")

	require.ErrorIs(t, err, edit.ErrAnchor)
	assert.Contains(t, err.Error(), "found a run of statements in registerHandlers that are each a call to *.Register 2 times")
}

const resolverSrc = `package graphql

type query struct {
	taskQuery
	exchangeQuery
}

func newResolver(b cqrs.Buses) *resolver {
	r := &resolver{}
	r.query.taskQuery, r.mutation.taskMutation = task.New(b)
	r.query.exchangeQuery = exchange.New(b)

	return r
}
`

func TestAppendFieldAddsToANamedStruct(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, resolverSrc).AppendField("query", "widgetQuery")
	require.NoError(t, err)
	assert.Contains(t, applyGo(t, resolverSrc, ins), "type query struct {\n\ttaskQuery\n\texchangeQuery\n\twidgetQuery\n}\n")

	_, err = parseGo(t, resolverSrc).AppendField("mutation", "widgetMutation")
	require.ErrorIs(t, err, edit.ErrAnchor)
}

// Each statement in newResolver fills in one domain's resolvers: the new one goes after the last.
func TestAfterLastStmtGoesAfterEveryAssignedCall(t *testing.T) {
	t.Parallel()

	ins, err := parseGo(t, resolverSrc).AfterLastStmt("newResolver", edit.Call("*.New"), "r.query.widgetQuery = widget.New(b)")
	require.NoError(t, err)
	assert.Contains(t, applyGo(t, resolverSrc, ins), "exchange.New(b)\n\tr.query.widgetQuery = widget.New(b)\n\n\treturn r\n")
}

func TestAppendElementAddsToTheOneLiteralOfItsType(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ src, want string }{
		"a multi-line list": {
			"package app\n\nfunc modules() []module {\n\treturn []module{\n\t\ttaskModule(),\n\t}\n}\n",
			"\t\ttaskModule(),\n\t\twidgetModule(),\n\t}\n",
		},
		"a one-line list": {
			"package app\n\nfunc modules() []module {\n\treturn []module{taskModule()}\n}\n",
			"[]module{taskModule(), widgetModule()}",
		},
	} {
		ins, err := parseGo(t, tc.src).AppendElement("modules", "[]module", "widgetModule()")
		require.NoError(t, err, name)
		assert.Contains(t, applyGo(t, tc.src, ins), tc.want, name)
	}

	twoLists := "package app\n\nfunc modules() []module {\n\t_ = []module{}\n\n\treturn []module{}\n}\n"
	_, err := parseGo(t, twoLists).AppendElement("modules", "[]module", "widgetModule()")
	require.ErrorIs(t, err, edit.ErrAnchor, "two literals")

	_, err = parseGo(t, twoLists).AppendElement("missing", "[]module", "widgetModule()")
	require.ErrorIs(t, err, edit.ErrAnchor, "no such func")
}

func TestApplyKeepsTheOrderOfInsertionsAtOneOffset(t *testing.T) {
	t.Parallel()

	got := edit.Apply([]byte("ad"), []edit.Insertion{{Offset: 1, Text: "b"}, {Offset: 1, Text: "c"}, {Offset: 0, Text: ">"}})

	assert.Equal(t, ">abcd", string(got))
}
