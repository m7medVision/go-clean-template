// Package rules holds the checks every plan passes before anything is written (R1 to R5 of the
// spec; R6 is the write itself, in package plan). Each check reads the repository and changes
// nothing.
package rules

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"slices"
)

// ErrTargets is a file to create that exists, a file to edit that does not, or a use case whose
// domain, message name or transport does not fit (rule R2).
var ErrTargets = errors.New("R2 targets")

// Repo is the repository a plan is made against: its files, read-only, and which of its paths
// have uncommitted changes.
type Repo struct {
	// FS is rooted at the repository root; paths are slash-separated and relative to it.
	FS fs.FS
	// Dirty returns the paths with uncommitted changes, untracked files included, among paths and
	// the files under them.
	Dirty func(ctx context.Context, paths []string) ([]string, error)
}

// Open returns the repository rooted at dir, with git telling which paths are dirty.
func Open(dir string) Repo {
	return Repo{FS: os.DirFS(dir), Dirty: func(ctx context.Context, paths []string) ([]string, error) {
		return gitDirty(ctx, dir, paths)
	}}
}

// Read returns the content of the file at p. A missing file is ErrTargets: a plan reads only the
// files it edits.
func (r Repo) Read(p string) ([]byte, error) {
	src, err := fs.ReadFile(r.FS, p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s, a file to edit, does not exist", ErrTargets, p)
	}

	if err != nil {
		return nil, fmt.Errorf("read %s: %w", p, err)
	}

	return src, nil
}

func (r Repo) exists(p string) bool {
	_, err := fs.Stat(r.FS, p)

	return err == nil
}

// glob returns the files matching pattern, which path.Match reads.
func (r Repo) glob(pattern string) []string {
	matches, err := fs.Glob(r.FS, pattern)
	if err != nil {
		return nil
	}

	return matches
}

// gitDirty runs git status in dir for paths and returns the paths it reports.
func gitDirty(ctx context.Context, dir string, paths []string) ([]string, error) {
	args := append([]string{"-C", dir, "status", "--porcelain", "--untracked-files=all", "--"}, paths...)

	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git status in %s: %w", dir, err)
	}

	var dirty []string

	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		// Each line is "XY path", or "XY old -> new" for a rename.
		if line := sc.Text(); len(line) > len("XY ") {
			dirty = append(dirty, path.Clean(line[len("XY "):]))
		}
	}

	slices.Sort(dirty)

	return dirty, nil
}
