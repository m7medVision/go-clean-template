package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errInjected = errors.New("injected failure")

// gitRepo is a committed repository holding the task domain's use-case folder and one shared
// file, internal/app/buses.go.
func gitRepo(t *testing.T) string {
	t.Helper()

	return gitRepoWith(t, map[string]string{
		"internal/app/buses.go":             "package app\n",
		"internal/usecase/task/register.go": "package task\n",
	})
}

// gitRepoWith is a repository holding files, by path, committed.
func gitRepoWith(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()

	for p, content := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(dir, p), []byte(content), 0o600))
	}

	git(t, dir, "init", "-q")
	commitAll(t, dir, "base")

	return dir
}

// commitAll commits everything in the working tree of the repository at dir.
func commitAll(t *testing.T, dir, message string) {
	t.Helper()

	git(t, dir, "add", "--all")
	git(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", message)
}

// git runs git with args in the repository at dir.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...) //nolint:gosec // fixed git commands on a temp dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

// fingerprint hashes every path, mode and content in the working tree under dir. .git is left
// out: git status refreshes its index.
func fingerprint(t *testing.T, dir string) string {
	t.Helper()

	var lines []string

	fsys := os.DirFS(dir)

	require.NoError(t, fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		line := fmt.Sprintf("%s %v", p, info.Mode())
		if !d.IsDir() {
			b, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}

			line += fmt.Sprintf(" %x", sha256.Sum256(b))
		}

		lines = append(lines, line)

		return nil
	}))

	sort.Strings(lines)

	return strings.Join(lines, "\n")
}

// widgetPlan creates a widget file and edits buses.go; it passes every rule.
func widgetPlan() *plan.Plan {
	p := plan.New("new-domain widget")
	p.Create("internal/usecase/widget/register.go", []byte("package widget\n"))
	p.Edit("internal/app/buses.go", []byte("package app\n"),
		edit.Insertion{Offset: len("package app\n"), Text: "\n// widget\n"})

	return p
}

func openRoot(t *testing.T, dir string) *os.Root {
	t.Helper()

	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	return root
}

func TestExecuteWritesAPlanThatPassesEveryRule(t *testing.T) {
	t.Parallel()

	dir := gitRepo(t)

	var out bytes.Buffer
	require.NoError(t, execute(t.Context(), rules.Open(dir), openRoot(t, dir), planned(widgetPlan(), nil), false, &out))

	got, err := os.ReadFile(filepath.Join(dir, "internal", "app", "buses.go"))
	require.NoError(t, err)
	assert.Equal(t, "package app\n\n// widget\n", string(got))
	assert.Contains(t, out.String(), "git checkout -- internal/app/buses.go docs/openapi/v1.json")
	assert.Contains(t, out.String(), "git clean -fd -- internal/usecase/widget docs/openapi/v1.json")
	assert.True(t, strings.HasSuffix(out.String(), "web/src/graphql\n\nNext:\n  - write it\n"), "the checklist, after the undo: %s", out.String())
}

// planned is the planner that made p, running into errs, with a checklist of one line.
func planned(p *plan.Plan, errs []error) *planner {
	return &planner{plan: p, errs: errs, next: "Next:\n  - write it\n"}
}

// ruleFailure sets up one rule's failure. arrange runs before the tree is fingerprinted: it is
// the state the user left. run is the command that must fail with want.
type ruleFailure struct {
	arrange func(t *testing.T, dir string)
	run     func(t *testing.T, dir string) error
	want    error
}

func ruleFailures() map[string]ruleFailure {
	return map[string]ruleFailure{
		"R1 name pattern": {nil, func(t *testing.T, dir string) error {
			t.Helper()

			return newDomain(t.Context(), &domainRequest{root: dir, name: "type"}, &bytes.Buffer{})
		}, names.ErrInvalid},
		"R1 existing domain": {nil, func(t *testing.T, dir string) error {
			t.Helper()

			return newDomain(t.Context(), &domainRequest{root: dir, name: "task"}, &bytes.Buffer{})
		}, names.ErrInvalid},
		"R2 file to create exists": {nil, func(t *testing.T, dir string) error {
			t.Helper()

			p := widgetPlan()
			p.Create("internal/usecase/task/register.go", []byte("package task\n"))

			return execute(t.Context(), rules.Open(dir), openRoot(t, dir), planned(p, nil), false, &bytes.Buffer{})
		}, rules.ErrTargets},
		"R3 file to edit is dirty": {func(t *testing.T, dir string) {
			t.Helper()

			require.NoError(t, os.WriteFile(filepath.Join(dir, "internal", "app", "buses.go"), []byte("package app // mine\n"), 0o600))
		}, func(t *testing.T, dir string) error {
			t.Helper()

			p := widgetPlan()

			return execute(t.Context(), rules.Open(dir), openRoot(t, dir), planned(p, nil), false, &bytes.Buffer{})
		}, rules.ErrClean},
		"R4 anchor not found": {nil, func(t *testing.T, dir string) error {
			t.Helper()

			return execute(t.Context(), rules.Open(dir), openRoot(t, dir), planned(widgetPlan(),
				[]error{fmt.Errorf("%w: buses.go: found func subscribers 0 times", edit.ErrAnchor)}), false, &bytes.Buffer{})
		}, edit.ErrAnchor},
		"R5 edit breaks a file": {nil, func(t *testing.T, dir string) error {
			t.Helper()

			p := widgetPlan()
			p.Edit("internal/app/buses.go", nil, edit.Insertion{Offset: 0, Text: "}"})

			return execute(t.Context(), rules.Open(dir), openRoot(t, dir), planned(p, nil), false, &bytes.Buffer{})
		}, plan.ErrInvalid},
		"R6 write fails part-way": {nil, func(t *testing.T, dir string) error {
			t.Helper()

			root := &failingRename{FS: openRoot(t, dir)}

			return execute(t.Context(), rules.Open(dir), root, planned(widgetPlan(), nil), false, &bytes.Buffer{})
		}, plan.ErrWrite},
	}
}

func TestEveryRuleFailureLeavesTheTreeByteIdentical(t *testing.T) {
	t.Parallel()

	for rule, tc := range ruleFailures() {
		t.Run(rule, func(t *testing.T) {
			t.Parallel()

			dir := gitRepo(t)
			if tc.arrange != nil {
				tc.arrange(t, dir)
			}

			before := fingerprint(t, dir)

			require.ErrorIs(t, tc.run(t, dir), tc.want)
			assert.Equal(t, before, fingerprint(t, dir))
		})
	}
}

// failingRename is a filesystem whose last rename of a plan fails, after the others succeeded.
type failingRename struct {
	plan.FS

	renames int
}

func (f *failingRename) Rename(oldname, newname string) error {
	f.renames++
	if f.renames == 2 {
		return errInjected
	}

	return f.FS.Rename(oldname, newname)
}
