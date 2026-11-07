package plan

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// ErrWrite is a write that failed part-way. Everything it had done is undone (rule R6).
var ErrWrite = errors.New("R6 all or nothing")

const (
	_tmpSuffix = ".gen-tmp"
	_fileMode  = 0o644
	_dirMode   = 0o755
)

// FS is what Write writes through, rooted at the repository root; an *os.Root is one, and keeps
// every write inside the root. Names are slash-separated.
type FS interface {
	Stat(name string) (fs.FileInfo, error)
	MkdirAll(name string, perm fs.FileMode) error
	WriteFile(name string, data []byte, perm fs.FileMode) error
	Rename(oldname, newname string) error
	Remove(name string) error
	RemoveAll(name string) error
}

// Write writes changes into root. Each file is written to a temporary file beside it first, and
// only once every one is written are they renamed into place. If any step fails, everything done
// is undone: edited files get their old content back, and created files and the folders made for
// them are removed.
func Write(root FS, changes []Change) error {
	w := &writer{root: root, pending: map[string]bool{}}

	if err := w.write(changes); err != nil {
		if undoErr := w.undo(); undoErr != nil {
			return fmt.Errorf("%w: %w; undoing it failed too, so the tree is half written: %w", ErrWrite, err, undoErr)
		}

		return fmt.Errorf("%w: %w; everything it wrote was undone", ErrWrite, err)
	}

	return nil
}

type writer struct {
	root FS
	// dirs are the folders made, outermost first.
	dirs []string
	// pending are temporary files written and not yet renamed into place.
	pending map[string]bool
	// placed are the changes renamed into place, with each edited file's mode.
	placed []placed
}

type placed struct {
	Change

	mode fs.FileMode
}

func (w *writer) write(changes []Change) error {
	modes := make([]fs.FileMode, len(changes))

	for i, c := range changes {
		mode, err := w.prepare(c)
		if err != nil {
			return err
		}

		modes[i] = mode
		w.pending[c.Path+_tmpSuffix] = true

		if err := w.root.WriteFile(c.Path+_tmpSuffix, c.New, mode); err != nil {
			return fmt.Errorf("write %s: %w", c.Path, err)
		}
	}

	for i, c := range changes {
		if err := w.root.Rename(c.Path+_tmpSuffix, c.Path); err != nil {
			return fmt.Errorf("move %s into place: %w", c.Path, err)
		}

		delete(w.pending, c.Path+_tmpSuffix)
		w.placed = append(w.placed, placed{Change: c, mode: modes[i]})
	}

	return nil
}

// prepare checks c still fits the tree and returns the mode to write it with: an edited file
// keeps its own, and a created file gets its folders made.
func (w *writer) prepare(c Change) (fs.FileMode, error) {
	info, err := w.root.Stat(c.Path)

	switch {
	case c.Old != nil && err != nil:
		return 0, fmt.Errorf("stat %s: %w", c.Path, err)
	case c.Old != nil:
		return info.Mode().Perm(), nil
	case err == nil:
		return 0, fmt.Errorf("%w: %s appeared since the plan was made", fs.ErrExist, c.Path)
	default:
		return _fileMode, w.mkdirs(path.Dir(c.Path))
	}
}

// mkdirs makes dir and any parent it lacks, one level at a time, remembering each it made.
func (w *writer) mkdirs(dir string) error {
	parts := strings.Split(dir, "/")

	for i := range parts {
		d := strings.Join(parts[:i+1], "/")
		if d == "." {
			continue
		}

		if _, err := w.root.Stat(d); err == nil {
			continue
		}

		if err := w.root.MkdirAll(d, _dirMode); err != nil {
			return fmt.Errorf("make %s: %w", d, err)
		}

		w.dirs = append(w.dirs, d)
	}

	return nil
}

// undo removes the temporary files, puts back what was renamed into place, and removes the folders
// made, innermost first.
func (w *writer) undo() error {
	var errs []error

	for tmp := range w.pending {
		if err := w.root.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}

	for _, p := range slices.Backward(w.placed) {
		if p.Old != nil {
			errs = append(errs, w.root.WriteFile(p.Path, p.Old, p.mode))
		} else {
			errs = append(errs, w.root.Remove(p.Path))
		}
	}

	for _, d := range slices.Backward(w.dirs) {
		errs = append(errs, w.root.RemoveAll(d))
	}

	return errors.Join(errs...)
}
