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
	"strings"
)

// ErrTargets is a file to create that exists, a file to edit that does not, or a use case whose
// domain, message name or transport does not fit (rule R2).
var ErrTargets = errors.New("R2 targets")

// Repo is the repository a plan is made against: its files, read-only, which of them have
// uncommitted changes, and the names of the app's packages.
type Repo struct {
	// FS is rooted at the repository root; paths are slash-separated and relative to it.
	FS fs.FS
	// Dirty returns the paths with uncommitted changes, untracked files included.
	Dirty func(ctx context.Context) ([]string, error)
	// Packages returns the names of the app's packages, each with the import path of one package
	// of that name.
	Packages func(ctx context.Context) (map[string]string, error)
}

// Open returns the repository rooted at dir, with git telling which paths are dirty and go list
// naming the packages.
func Open(dir string) Repo {
	return Repo{
		FS:       os.DirFS(dir),
		Dirty:    func(ctx context.Context) ([]string, error) { return gitDirty(ctx, dir) },
		Packages: func(ctx context.Context) (map[string]string, error) { return goPackages(ctx, dir) },
	}
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

// gitDirty runs git status in dir and returns the paths it reports under dir.
func gitDirty(ctx context.Context, dir string) ([]string, error) {
	out, err := command(ctx, "git", "-C", dir, "status", "--porcelain", "--untracked-files=all", "--", ".")
	if err != nil {
		return nil, err
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

// goPackages runs go list in dir and returns the module's packages by name, each with the import
// path of the first package of that name. -find leaves imports unresolved, so no module needs to
// be downloaded; -e lists a package that has errors too.
func goPackages(ctx context.Context, dir string) (map[string]string, error) {
	out, err := command(ctx, "go", "-C", dir, "list", "-find", "-e", "-f", "{{.Name}} {{.ImportPath}}", "./...")
	if err != nil {
		return nil, err
	}

	pkgs := map[string]string{}

	for line := range strings.Lines(string(out)) {
		name, importPath, ok := strings.Cut(strings.TrimSpace(line), " ")
		if _, seen := pkgs[name]; ok && name != "" && !seen {
			pkgs[name] = importPath
		}
	}

	return pkgs, nil
}

// command runs name with args and returns what it prints. A failure says what it printed on
// stderr.
func command(ctx context.Context, name string, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()

	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, bytes.TrimSpace(exit.Stderr))
	}

	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}

	return out, nil
}
