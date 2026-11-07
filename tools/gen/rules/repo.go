// Package rules holds the checks every plan passes before anything is written (R1 to R5 of the
// spec; R6 is the write itself, in package plan). Each check reads the repository and changes
// nothing.
package rules

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// ErrTargets is a file to create that exists, a file to edit that does not, or a use case whose
// domain, message name or transport does not fit (rule R2).
var ErrTargets = errors.New("R2 targets")

// errPorcelain is git status output that is not what --porcelain -z prints.
var errPorcelain = errors.New("git status printed an entry it does not document")

// errNotTopLevel is a root that is not the top level of its git repository: git reports paths
// from the top level, and the plan's are from the root.
var errNotTopLevel = errors.New("is not the top level of its git repository: run the generator from there")

// Repo is the repository a plan is made against: its files, read-only, what git says of them, and
// the names of the app's packages.
type Repo struct {
	// FS is rooted at the repository root; paths are slash-separated and relative to it.
	FS fs.FS
	// Status returns what git status says of the working tree.
	Status func(ctx context.Context) (Status, error)
	// Packages returns the names of the app's packages, each with the import path of one package
	// of that name.
	Packages func(ctx context.Context) (map[string]string, error)
}

// Status is what git status says of the working tree. Paths are relative to the repository root.
type Status struct {
	// Changed are the tracked files with uncommitted changes, staged or not, both paths of a
	// rename among them.
	Changed []string
	// Untracked are the paths git neither tracks nor ignores. A folder that holds nothing else is
	// one path, ending in a slash.
	Untracked []string
}

// Open returns the repository rooted at dir, with git saying what changed and go list naming the
// packages.
func Open(dir string) Repo {
	return Repo{
		FS:       os.DirFS(dir),
		Status:   func(ctx context.Context) (Status, error) { return gitStatus(ctx, dir) },
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

// Exists reports whether the repository has a file or folder at p.
func (r Repo) Exists(p string) bool {
	_, err := fs.Stat(r.FS, p)

	return err == nil
}

// glob returns the files matching pattern, which path.Match reads.
func (r Repo) glob(pattern string) ([]string, error) {
	matches, err := fs.Glob(r.FS, pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	return matches, nil
}

// gitStatus runs git status in dir, which must be the top level of its repository, since git
// reports every path from there.
func gitStatus(ctx context.Context, dir string) (Status, error) {
	prefix, err := command(ctx, "git", "-C", dir, "rev-parse", "--show-prefix")
	if err != nil {
		return Status{}, err
	}

	if len(bytes.TrimSpace(prefix)) > 0 {
		return Status{}, fmt.Errorf("%s %w", dir, errNotTopLevel)
	}

	out, err := command(ctx, "git", "-C", dir, "status", "--porcelain", "-z", "--untracked-files=normal")
	if err != nil {
		return Status{}, err
	}

	return porcelain(out)
}

// porcelain reads out, what git status --porcelain -z prints: one "XY path" entry after another,
// each ended by a NUL, where XY is ?? for an untracked path, and after a rename or a copy, the path
// it came from as one more NUL-ended field. Both paths of a rename are changed. -z prints every
// path as it is, never quoted.
func porcelain(out []byte) (Status, error) {
	var st Status

	if len(out) == 0 {
		return st, nil
	}

	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")

	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) <= len("XY ") {
			return Status{}, fmt.Errorf("%w: %q", errPorcelain, entry)
		}

		xy, p := entry[:len("XY")], entry[len("XY "):]
		if xy == "??" {
			st.Untracked = append(st.Untracked, p)

			continue
		}

		st.Changed = append(st.Changed, p)

		if strings.ContainsAny(xy, "RC") {
			i++
			if i == len(fields) {
				return Status{}, fmt.Errorf("%w: %q has no path it came from", errPorcelain, entry)
			}

			st.Changed = append(st.Changed, fields[i])
		}
	}

	slices.Sort(st.Changed)
	slices.Sort(st.Untracked)
	st.Changed = slices.Compact(st.Changed)

	return st, nil
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
