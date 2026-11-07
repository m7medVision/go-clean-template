package rules_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitRepo is a repository with a.go and gen/out.json committed.
func gitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "gen"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gen", "out.json"), []byte("{}\n"), 0o600))

	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "base"},
	} {
		git(t, dir, args...)
	}

	return dir
}

// _r3 is how R3's failures start: the rule's name, as the generator prints it.
const _r3 = "R3 clean: "

// checkTree runs the rules on a plan that changes nothing, in the repository at dir: of them only
// R3 can fail.
func checkTree(t *testing.T, dir string) error {
	t.Helper()

	return checkIn(t, dir, plan.New("x"))
}

// checkIn runs the rules on p, which must pass every rule but R3, in the repository at dir.
func checkIn(t *testing.T, dir string, p *plan.Plan) error {
	t.Helper()

	return rules.Check(t.Context(), rules.Open(dir), p, nil)
}

// checkPlan runs the rules on p in an empty, clean repository: of them only R5 can fail.
func checkPlan(t *testing.T, p *plan.Plan) error {
	t.Helper()

	return rules.Check(t.Context(), repoOf(fstest.MapFS{}), p, nil)
}

// git runs git with args in the repository at dir.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...).CombinedOutput() //nolint:gosec // fixed git commands on a temp dir
	require.NoError(t, err, string(out))
}

func TestCleanPassesOnACleanTree(t *testing.T) {
	t.Parallel()

	require.NoError(t, checkTree(t, gitRepo(t)))
}

// R3 covers every tracked file, not only those the plan edits: the regenerators may change any of
// them, and the undo restores them all. A change counts staged or not. A rename names both of its
// paths, and a path git would quote names itself as it is.
func TestCleanRefusesAnyChangedTrackedFile(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		change func(t *testing.T, dir string)
		paths  string
	}{
		"an edited file": {
			func(t *testing.T, dir string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package b\n"), 0o600))
			},
			"a.go",
		},
		"a staged file": {
			func(t *testing.T, dir string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "gen", "out.json"), []byte("[]\n"), 0o600))
				git(t, dir, "add", "gen/out.json")
			},
			"gen/out.json",
		},
		"a file renamed to a path git quotes": {
			func(t *testing.T, dir string) {
				t.Helper()
				git(t, dir, "mv", "a.go", `naïve "name".go`)
			},
			`a.go, naïve "name".go`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := gitRepo(t)
			tc.change(t, dir)

			err := checkTree(t, dir)
			require.ErrorContains(t, err, _r3)
			assert.True(t, strings.HasSuffix(err.Error(), ": "+tc.paths), "%s does not end in the paths %s", err, tc.paths)
		})
	}
}

// Untracked files may be there, since the undo leaves each of them alone; but not where the plan
// writes: the undo could not take back an edit to an untracked file, nor tell a file the plan
// created in an untracked folder from those already there.
func TestCleanAcceptsUntrackedFilesTheUndoLeavesAlone(t *testing.T) {
	t.Parallel()

	dir := gitRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "tasks"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tasks", "todo.md"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gen", "notes.md"), nil, 0o600))

	require.NoError(t, checkTree(t, dir))

	elsewhere := plan.New("x")
	elsewhere.Create("gen/new.go", []byte("package gen\n"))
	require.NoError(t, checkIn(t, dir, elsewhere))

	editing := plan.New("x")
	editing.Edit("gen/notes.md", nil)

	err := checkIn(t, dir, editing)
	require.ErrorContains(t, err, _r3)
	assert.Contains(t, err.Error(), "gen/notes.md is untracked")

	creating := plan.New("x")
	creating.Create("tasks/new.go", []byte("package tasks\n"))

	err = checkIn(t, dir, creating)
	require.ErrorContains(t, err, _r3)
	assert.Contains(t, err.Error(), "tasks/ is untracked, so the undo could not take tasks/new.go back")
}

// The undo excludes each untracked path by name, anchored at the root, its pattern characters
// escaped and the whole quoted for the shell where it needs it.
func TestUndoExcludesEveryUntrackedPath(t *testing.T) {
	t.Parallel()

	r := rules.Repo{Status: func(context.Context) (rules.Status, error) {
		return rules.Status{Untracked: []string{"tasks/", "my notes [old].md", "it's.txt"}}, nil
	}}

	got, err := rules.Undo(t.Context(), r)

	require.NoError(t, err)
	assert.Equal(t, `git checkout -- . && git clean -fd -e /tasks/ -e '/my notes \[old].md' -e '/it'\''s.txt' -- .`, got)
}

// git reports paths from the top level, and the plan's are from the root, so the two must be one.
func TestCleanRefusesARootBelowTheTopLevel(t *testing.T) {
	t.Parallel()

	err := checkTree(t, filepath.Join(gitRepo(t), "gen"))

	require.ErrorContains(t, err, _r3)
	assert.Contains(t, err.Error(), "is not the top level of its git repository")
}

func TestCleanRefusesADirectoryThatIsNotARepository(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, checkTree(t, t.TempDir()), _r3)
}

func TestValidAcceptsFilesThatParse(t *testing.T) {
	t.Parallel()

	p := plan.New("x")
	p.Create("a.go", []byte("package a\n"))
	p.Create("a.proto", []byte("syntax = \"proto3\";\n"))
	p.Create("a.graphqls", []byte("type A { id: ID! }\n"))
	p.Create("a.sql", []byte("not checked"))
	p.Edit("b.go", []byte("package b\n"), edit.Insertion{Offset: len("package b\n"), Text: "\nconst c = 1\n"})

	require.NoError(t, checkPlan(t, p))
}

func TestValidRefusesWhatNoLongerParses(t *testing.T) {
	t.Parallel()

	for name, create := range map[string][2]string{
		"Go":      {"a.go", "package a\n\nfunc {"},
		"proto":   {"a.proto", "service {"},
		"GraphQL": {"a.graphqls", "type A {"},
	} {
		p := plan.New("x")
		p.Create(create[0], []byte(create[1]))

		require.ErrorIs(t, checkPlan(t, p), plan.ErrInvalid, name)
	}

	p := plan.New("x")
	p.Edit("b.go", []byte("package b\n"), edit.Insertion{Offset: 0, Text: "}"})

	require.ErrorIs(t, checkPlan(t, p), plan.ErrInvalid, "an edit that breaks a Go file")
}

func TestCheckReportsTheFirstRuleThatFails(t *testing.T) {
	t.Parallel()

	dirty := rules.Repo{
		FS:     fstest.MapFS{"a.go": {Data: []byte("package a\n")}},
		Status: func(context.Context) (rules.Status, error) { return rules.Status{Changed: []string{"a.go"}}, nil },
	}
	clean := rules.Repo{FS: dirty.FS, Status: func(context.Context) (rules.Status, error) { return rules.Status{}, nil }}

	creatingA := plan.New("x")
	creatingA.Create("a.go", []byte("package a\n"))

	editingA := plan.New("x")
	editingA.Edit("a.go", []byte("package a\n"))

	brokenEdit := plan.New("x")
	brokenEdit.Edit("a.go", []byte("package a\n"), edit.Insertion{Offset: 0, Text: "}"})

	anchor := []error{edit.ErrAnchor}
	missing := []error{rules.ErrTargets}

	// Each rule's failure starts with the rule's name, as the generator prints it.
	for name, tc := range map[string]struct {
		r        rules.Repo
		p        *plan.Plan
		planErrs []error
		want     string
	}{
		"R2 before R3 and R4":  {dirty, creatingA, anchor, "R2 targets: "},
		"a missing file is R2": {dirty, editingA, append(missing, anchor...), "R2 targets"},
		"R3 before R4":         {dirty, editingA, anchor, _r3},
		"R4 before R5":         {clean, brokenEdit, anchor, "R4 one anchor"},
		"R5 last":              {clean, brokenEdit, nil, "R5 still valid: "},
		"everything passes":    {clean, editingA, nil, ""},
	} {
		err := rules.Check(t.Context(), tc.r, tc.p, tc.planErrs)
		if tc.want == "" {
			require.NoError(t, err, name)

			continue
		}

		require.Error(t, err, name)
		assert.True(t, strings.HasPrefix(err.Error(), tc.want), "%s: %s is not %s", name, err, tc.want)
	}
}

func TestCheckReturnsAnUnexpectedErrorFirst(t *testing.T) {
	t.Parallel()

	broken := errors.New("disk on fire") //nolint:err113 // a one-off error is the point

	err := rules.Check(t.Context(), repoOf(fixtureRoot()), plan.New("x"), []error{edit.ErrAnchor, broken})
	require.ErrorIs(t, err, broken)
}
