package plan_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errInjected = errors.New("injected failure")

// tree is a directory holding two files to edit, both 0600, and the
// empty folders internal/entity and internal/usecase.
func tree(t *testing.T) (string, *os.Root) {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal", "app"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal", "entity"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal", "usecase"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "internal", "app", "buses.go"), []byte("package app\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# x\n"), 0o600))

	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	return dir, root
}

// fingerprint hashes every path, mode and content under dir, so two calls compare the whole tree.
func fingerprint(t *testing.T, dir string) string {
	t.Helper()

	var lines []string

	fsys := os.DirFS(dir)

	require.NoError(t, fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
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

func changes() []plan.Change {
	return []plan.Change{
		{Path: "internal/entity/widget.go", New: []byte("package entity\n")},
		{Path: "internal/usecase/widget/command/create_widget.go", New: []byte("package command\n")},
		{Path: "internal/app/buses.go", Old: []byte("package app\n"), New: []byte("package app\n\n// edited\n")},
		{Path: "README.md", Old: []byte("# x\n"), New: []byte("# x\n\nedited\n")},
	}
}

func TestWriteWritesEveryChange(t *testing.T) {
	t.Parallel()

	dir, root := tree(t)

	require.NoError(t, plan.Write(root, changes()))

	got, err := os.ReadFile(filepath.Join(dir, "internal", "usecase", "widget", "command", "create_widget.go"))
	require.NoError(t, err)
	assert.Equal(t, "package command\n", string(got))

	got, err = os.ReadFile(filepath.Join(dir, "internal", "app", "buses.go"))
	require.NoError(t, err)
	assert.Equal(t, "package app\n\n// edited\n", string(got))

	info, err := os.Stat(filepath.Join(dir, "internal", "app", "buses.go"))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm(), "an edited file keeps its mode, not the 0644 of a new one")

	entries, err := os.ReadDir(filepath.Join(dir, "internal", "app"))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary file is left behind")
}

func TestWriteUndoesEverythingWhenAStepFails(t *testing.T) {
	t.Parallel()

	steps := len(changes()) * 2 // a temporary file and a rename for each change

	for failAt := 1; failAt <= steps; failAt++ {
		t.Run(fmt.Sprint("step ", failAt), func(t *testing.T) {
			t.Parallel()

			dir, root := tree(t)
			before := fingerprint(t, dir)

			err := plan.Write(&failing{FS: root, failAt: failAt}, changes())
			require.ErrorIs(t, err, plan.ErrWrite)
			require.ErrorIs(t, err, errInjected)

			assert.Equal(t, before, fingerprint(t, dir))
		})
	}
}

// failing is a filesystem whose failAt-th write or rename fails.
type failing struct {
	plan.FS

	failAt int
	steps  int
}

func (f *failing) step() error {
	f.steps++
	if f.steps == f.failAt {
		return errInjected
	}

	return nil
}

func (f *failing) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := f.step(); err != nil {
		return err
	}

	return f.FS.WriteFile(name, data, perm)
}

func (f *failing) Rename(oldname, newname string) error {
	if err := f.step(); err != nil {
		return err
	}

	return f.FS.Rename(oldname, newname)
}
