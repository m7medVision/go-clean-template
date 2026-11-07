package rules_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...) //nolint:gosec // fixed git commands on a temp dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	return dir
}

func TestCleanPassesOnACleanTree(t *testing.T) {
	t.Parallel()

	dir := gitRepo(t)

	require.NoError(t, rules.Clean(t.Context(), rules.Open(dir)))
}

// R3 covers the whole tree, not only the files the plan edits: the regenerators may change any
// of them, and the undo restores every change git reports.
func TestCleanRefusesAnyChangedOrUntrackedFile(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		change func(dir string) error
		path   string
	}{
		"an edited file": {
			func(dir string) error { return os.WriteFile(filepath.Join(dir, "a.go"), []byte("package b\n"), 0o600) },
			"a.go",
		},
		"an untracked file": {
			func(dir string) error { return os.WriteFile(filepath.Join(dir, "gen", "new.json"), nil, 0o600) },
			"gen/new.json",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := gitRepo(t)
			require.NoError(t, tc.change(dir))

			err := rules.Clean(t.Context(), rules.Open(dir))
			require.ErrorIs(t, err, rules.ErrClean)
			assert.Contains(t, err.Error(), tc.path)
		})
	}
}

func TestCleanRefusesADirectoryThatIsNotARepository(t *testing.T) {
	t.Parallel()

	err := rules.Clean(t.Context(), rules.Open(t.TempDir()))
	require.ErrorIs(t, err, rules.ErrClean)
}

func TestValidAcceptsFilesThatParse(t *testing.T) {
	t.Parallel()

	p := plan.New("x")
	p.Create("a.go", []byte("package a\n"))
	p.Create("a.proto", []byte("syntax = \"proto3\";\n"))
	p.Create("a.graphqls", []byte("type A { id: ID! }\n"))
	p.Create("a.sql", []byte("not checked"))
	p.Edit("gqlgen.yml", []byte("models:\n"), edit.Insertion{Offset: len("models:\n"), Text: "  A:\n    model: x.A\n"})

	require.NoError(t, rules.Valid(p))
}

func TestValidRefusesWhatNoLongerParses(t *testing.T) {
	t.Parallel()

	for name, create := range map[string][2]string{
		"Go":             {"a.go", "package a\n\nfunc {"},
		"unformatted Go": {"a.go", "package a\nfunc f() {  }\n"},
		"proto":          {"a.proto", "service {"},
		"GraphQL":        {"a.graphqls", "type A {"},
		"YAML":           {"a.yml", "a: [b"},
	} {
		p := plan.New("x")
		p.Create(create[0], []byte(create[1]))

		require.ErrorIs(t, rules.Valid(p), plan.ErrInvalid, name)
	}

	p := plan.New("x")
	p.Edit("b.go", []byte("package b\n"), edit.Insertion{Offset: 0, Text: "}"})

	require.ErrorIs(t, rules.Valid(p), plan.ErrInvalid, "an edit that breaks a Go file")
}

func TestCheckReportsTheFirstRuleThatFails(t *testing.T) {
	t.Parallel()

	dirty := rules.Repo{
		FS:    fstest.MapFS{"a.go": {Data: []byte("package a\n")}},
		Dirty: func(context.Context) ([]string, error) { return []string{"a.go"}, nil },
	}
	clean := rules.Repo{FS: dirty.FS, Dirty: func(context.Context) ([]string, error) { return nil, nil }}

	creatingA := plan.New("x")
	creatingA.Create("a.go", []byte("package a\n"))

	editingA := plan.New("x")
	editingA.Edit("a.go", []byte("package a\n"))

	brokenEdit := plan.New("x")
	brokenEdit.Edit("a.go", []byte("package a\n"), edit.Insertion{Offset: 0, Text: "}"})

	anchor := []error{edit.ErrAnchor}
	missing := []error{rules.ErrTargets}

	for name, tc := range map[string]struct {
		r        rules.Repo
		p        *plan.Plan
		planErrs []error
		want     error
	}{
		"R2 before R3 and R4":  {dirty, creatingA, anchor, rules.ErrTargets},
		"a missing file is R2": {dirty, editingA, append(missing, anchor...), rules.ErrTargets},
		"R3 before R4":         {dirty, editingA, anchor, rules.ErrClean},
		"R4 before R5":         {clean, brokenEdit, anchor, edit.ErrAnchor},
		"R5 last":              {clean, brokenEdit, nil, plan.ErrInvalid},
		"everything passes":    {clean, editingA, nil, nil},
	} {
		err := rules.Check(t.Context(), tc.r, tc.p, tc.planErrs)
		if tc.want == nil {
			require.NoError(t, err, name)

			continue
		}

		require.ErrorIs(t, err, tc.want, name)

		for _, other := range []error{rules.ErrTargets, rules.ErrClean, edit.ErrAnchor, plan.ErrInvalid} {
			if !errors.Is(other, tc.want) {
				assert.NotErrorIs(t, err, other, name)
			}
		}
	}
}

func TestCheckReturnsAnUnexpectedErrorFirst(t *testing.T) {
	t.Parallel()

	broken := errors.New("disk on fire") //nolint:err113 // a one-off error is the point

	err := rules.Check(t.Context(), repoOf(fixtureRoot()), plan.New("x"), []error{edit.ErrAnchor, broken})
	require.ErrorIs(t, err, broken)
}
