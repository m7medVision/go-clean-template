package scaffold

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// _binarySniff is how much of a file is searched for a NUL byte to tell binary from text.
const _binarySniff = 8000

// _dirPerm is the mode of the directories Generate creates.
const _dirPerm = 0o755

// ErrOutputExists is an output directory that is not empty, or that holds the template itself.
var ErrOutputExists = errors.New("output directory is not usable")

// Options configure one Generate run.
type Options struct {
	Manifest *Manifest
	Keep     Set
	// Source is the template's root.
	Source string
	// Files lists the template's files, relative and slash-separated. Empty means every file git
	// tracks or would track (so uncommitted changes are included).
	Files []string
	// Out is the directory the project is written to.
	Out string
	// Module replaces the manifest's module path; empty keeps it.
	Module string
	// Force replaces an output directory that is not empty.
	Force bool
}

// Report counts what Generate copied.
type Report struct {
	Kept, Removed int
}

// Generate writes the project the options describe to Out.
func Generate(ctx context.Context, opts *Options) (Report, error) {
	if err := prepareOut(opts); err != nil {
		return Report{}, err
	}

	files := opts.Files
	if len(files) == 0 {
		var err error

		files, err = ListFiles(ctx, opts.Source)
		if err != nil {
			return Report{}, err
		}
	}

	rules := Rules{Manifest: opts.Manifest, Keep: opts.Keep}

	var report Report

	for _, name := range files {
		if !opts.Manifest.Keeps(opts.Keep, name) {
			report.Removed++

			continue
		}

		if err := copyFile(opts, rules, name); err != nil {
			return Report{}, err
		}

		report.Kept++
	}

	return report, nil
}

// Keeps reports whether a project with keep gets the file name: not the scaffolder's, and every
// feature that claims it is kept.
func (m *Manifest) Keeps(keep Set, name string) bool {
	if matchAny(m.Self, name) {
		return false
	}

	for i := range m.Features {
		f := &m.Features[i]
		if !keep.Has(f.ID) && matchAny(f.Paths, name) {
			return false
		}
	}

	return true
}

// ListFiles lists what git tracks under root plus the untracked files it does not ignore. When root
// is not the top of a git work tree (a downloaded source archive), it lists every file instead.
func ListFiles(ctx context.Context, root string) ([]string, error) {
	if !isWorkTreeTop(ctx, root) {
		return walkFiles(root)
	}

	cmd := exec.CommandContext(ctx, "git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = root

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("scaffold - ListFiles - git ls-files: %w", err)
	}

	var files []string

	for name := range strings.SplitSeq(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		// A tracked file deleted in the working tree is still listed; skip it.
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); err == nil {
			files = append(files, name)
		}
	}

	return files, nil
}

// isWorkTreeTop reports whether root is the top of a git work tree. A directory nested in an
// unrelated repository (an archive extracted under one) is not: that repository's index and
// ignore rules say nothing about it.
func isWorkTreeTop(ctx context.Context, root string) bool {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = root

	out, err := cmd.Output()
	if err != nil {
		return false
	}

	top, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return false
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return false
	}

	abs, err = filepath.EvalSymlinks(abs)

	return err == nil && top == abs
}

// walkFiles lists every file and symlink under root, sorted. A source archive holds exactly the
// tracked files, so this is what git would list for it.
func walkFiles(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		files = append(files, filepath.ToSlash(rel))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scaffold - ListFiles - filepath.WalkDir: %w", err)
	}

	slices.Sort(files)

	return files, nil
}

// prepareOut makes sure Out is empty (or emptied, with Force) and is neither the template nor one
// of its parents.
func prepareOut(opts *Options) error {
	src, err := filepath.Abs(opts.Source)
	if err != nil {
		return fmt.Errorf("scaffold - Generate - filepath.Abs: %w", err)
	}

	out, err := filepath.Abs(opts.Out)
	if err != nil {
		return fmt.Errorf("scaffold - Generate - filepath.Abs: %w", err)
	}

	if rel, err := filepath.Rel(out, src); err == nil && !strings.HasPrefix(rel, "..") {
		return fmt.Errorf("%w: %s holds the template", ErrOutputExists, opts.Out)
	}

	return emptyOut(out, opts.Force)
}

// emptyOut creates out, or empties it when force is set; a non-empty out is otherwise an error.
func emptyOut(out string, force bool) error {
	entries, err := os.ReadDir(out)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("scaffold - Generate - os.ReadDir: %w", err)
	}

	if len(entries) > 0 {
		if !force {
			return fmt.Errorf("%w: %s is not empty (use --force to replace it)", ErrOutputExists, out)
		}

		if err := os.RemoveAll(out); err != nil {
			return fmt.Errorf("scaffold - Generate - os.RemoveAll: %w", err)
		}
	}

	if err := os.MkdirAll(out, _dirPerm); err != nil {
		return fmt.Errorf("scaffold - Generate - os.MkdirAll: %w", err)
	}

	return nil
}

func copyFile(opts *Options, rules Rules, name string) error {
	if !filepath.IsLocal(filepath.FromSlash(name)) {
		return fmt.Errorf("%w: %q is not a path inside the template", ErrOutputExists, name)
	}

	src := filepath.Join(opts.Source, filepath.FromSlash(name))
	dst := filepath.Join(opts.Out, filepath.FromSlash(name))

	if err := os.MkdirAll(filepath.Dir(dst), _dirPerm); err != nil {
		return fmt.Errorf("scaffold - Generate - os.MkdirAll: %w", err)
	}

	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("scaffold - Generate - os.Lstat: %w", err)
	}

	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return fmt.Errorf("scaffold - Generate - os.Readlink: %w", err)
		}

		if err := os.Symlink(target, dst); err != nil {
			return fmt.Errorf("scaffold - Generate - os.Symlink: %w", err)
		}

		return nil
	}

	content, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("scaffold - Generate - os.ReadFile: %w", err)
	}

	content, err = transform(opts, rules, name, content)
	if err != nil {
		return err
	}

	// name passed filepath.IsLocal above, so dst stays inside Out.
	if err := os.WriteFile(dst, content, info.Mode().Perm()); err != nil { //nolint:gosec // see above
		return fmt.Errorf("scaffold - Generate - os.WriteFile: %w", err)
	}

	return nil
}

// transform strips a text file's markers and renames the module in it. Binary files pass through.
func transform(opts *Options, rules Rules, name string, content []byte) ([]byte, error) {
	if bytes.IndexByte(content[:min(len(content), _binarySniff)], 0) >= 0 {
		return content, nil
	}

	content, err := rules.Strip(name, content)
	if err != nil {
		return nil, err
	}

	if opts.Module != "" && opts.Module != opts.Manifest.Module {
		content = bytes.ReplaceAll(content, []byte(opts.Manifest.Module), []byte(opts.Module))
	}

	return content, nil
}
