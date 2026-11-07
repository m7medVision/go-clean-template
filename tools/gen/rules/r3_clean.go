package rules

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrClean is a working tree with uncommitted changes (rule R3).
var ErrClean = errors.New("R3 clean")

// Clean checks R3: no file in the working tree has uncommitted changes, untracked files included.
// So once the plan is written and the regenerators have run, every change git reports is theirs,
// and the undo restores every one of them.
func Clean(ctx context.Context, r Repo) error {
	dirty, err := r.Dirty(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrClean, err)
	}

	if len(dirty) > 0 {
		return fmt.Errorf("%w: commit or stash these first, so the undo is exact: %s", ErrClean, strings.Join(dirty, ", "))
	}

	return nil
}
