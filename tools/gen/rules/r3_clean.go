package rules

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// errClean is a working tree with uncommitted changes (rule R3).
var errClean = errors.New("R3 clean")

// clean checks R3: no file in the working tree has uncommitted changes, untracked files included.
// So once the plan is written and the regenerators have run, every change git reports is theirs,
// and the undo restores every one of them.
func clean(ctx context.Context, r Repo) error {
	dirty, err := r.Dirty(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", errClean, err)
	}

	if len(dirty) > 0 {
		return fmt.Errorf("%w: commit or stash these first, so the undo is exact: %s", errClean, strings.Join(dirty, ", "))
	}

	return nil
}
