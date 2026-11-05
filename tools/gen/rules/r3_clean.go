package rules

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrClean is a file to edit, or a regenerator's output, with uncommitted changes (rule R3).
var ErrClean = errors.New("R3 clean")

// Clean checks R3: none of paths, nor any file under them, has uncommitted changes, untracked
// files included. That is what makes the printed undo exact.
func Clean(ctx context.Context, r Repo, paths []string) error {
	dirty, err := r.Dirty(ctx, paths)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrClean, err)
	}

	if len(dirty) > 0 {
		return fmt.Errorf("%w: commit or stash these first, so the undo is exact: %s", ErrClean, strings.Join(dirty, ", "))
	}

	return nil
}
