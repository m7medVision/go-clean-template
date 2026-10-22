package scaffold_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir

	out, err := cmd.Output()
	require.NoError(t, err, "git %s", strings.Join(args, " "))

	return string(out)
}

// TestListFilesWalksADirectoryThatIsNotAGitRepository covers a downloaded source archive: no .git,
// so every file and symlink is listed, directories are not.
func TestListFilesWalksADirectoryThatIsNotAGitRepository(t *testing.T) {
	t.Parallel()

	root, _ := writeTree(t, map[string]string{
		"README.md":         "hi\n",
		"cmd/app/main.go":   "package main\n",
		"web/src/routes/a":  "a\n",
		".github/ci.yml":    "on: push\n",
		"empty/dir/.keep":   "",
		"internal/x/y/z.go": "package y\n",
	})
	require.NoError(t, os.Symlink("README.md", filepath.Join(root, "AGENTS.md")))

	files, err := scaffold.ListFiles(t.Context(), root)
	require.NoError(t, err)

	assert.Equal(t, []string{
		".github/ci.yml", "AGENTS.md", "README.md", "cmd/app/main.go", "empty/dir/.keep",
		"internal/x/y/z.go", "web/src/routes/a",
	}, files)
}

// TestListFilesWalksADirectoryNestedInAnotherRepository covers an archive extracted somewhere that
// is itself inside an unrelated git work tree: that repository's index says nothing about it.
func TestListFilesWalksADirectoryNestedInAnotherRepository(t *testing.T) {
	t.Parallel()

	outer := t.TempDir()
	git(t, outer, "init", "--quiet")
	require.NoError(t, os.WriteFile(filepath.Join(outer, ".gitignore"), []byte("template/\n"), 0o600))

	root := filepath.Join(outer, "template")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte("package pkg\n"), 0o600))

	files, err := scaffold.ListFiles(t.Context(), root)
	require.NoError(t, err)

	assert.Equal(t, []string{"pkg/a.go"}, files)
}

// TestListFilesOfASourceArchiveMatchesGit proves the walk lists what git tracks: the template
// archived at HEAD, as GitHub serves a tag, lists exactly HEAD's files.
func TestListFilesOfASourceArchiveMatchesGit(t *testing.T) {
	t.Parallel()

	archive := exec.CommandContext(t.Context(), "git", "archive", "--format=tar", "HEAD")
	archive.Dir = _repoRoot

	tarball, err := archive.Output()
	require.NoError(t, err)

	dir := t.TempDir()
	untar := exec.CommandContext(t.Context(), "tar", "-x", "-C", dir)
	untar.Stdin = bytes.NewReader(tarball)
	require.NoError(t, untar.Run())

	files, err := scaffold.ListFiles(t.Context(), dir)
	require.NoError(t, err)

	tracked := strings.Split(strings.TrimSuffix(git(t, _repoRoot, "ls-tree", "-r", "--name-only", "-z", "HEAD"), "\x00"), "\x00")
	slices.Sort(tracked)

	assert.Equal(t, tracked, files)
}
