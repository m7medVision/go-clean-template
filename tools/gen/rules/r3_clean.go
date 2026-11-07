package rules

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/plan"
)

// errClean is a working tree whose state the undo could not restore (rule R3).
var errClean = errors.New("R3 clean")

// shellSafe matches a word a shell reads as it is, which needs no quotes.
var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

// clean checks R3: no tracked file has uncommitted changes, staged or not, and the plan p neither
// edits an untracked file nor creates one in an untracked folder. Untracked files may be there:
// the undo leaves each of them alone. So once the plan is written and the regenerators have run,
// git checkout gives back every tracked file, and git clean removes only what the run added.
func clean(ctx context.Context, r Repo, p *plan.Plan) error {
	st, err := r.Status(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", errClean, err)
	}

	if len(st.Changed) > 0 {
		return fmt.Errorf("%w: commit or stash these first, so the undo is exact: %s", errClean, strings.Join(st.Changed, ", "))
	}

	for _, f := range slices.Concat(p.Edited(), p.Created()) {
		for _, u := range st.Untracked {
			if f == u || strings.HasSuffix(u, "/") && strings.HasPrefix(f, u) {
				return fmt.Errorf("%w: %s is untracked, so the undo could not take %s back: commit it or move it first",
					errClean, u, f)
			}
		}
	}

	return nil
}

// Undo returns the command that takes the working tree back to how it is now, once the plan is
// written and the regenerators have run, whatever they changed: git checkout restores every
// tracked file, and git clean removes every untracked one but those untracked now, each excluded
// by name. It is exact because R3 held. git clean leaves files git ignores alone.
func Undo(ctx context.Context, r Repo) (string, error) {
	st, err := r.Status(ctx)
	if err != nil {
		return "", fmt.Errorf("the undo: %w", err)
	}

	var b strings.Builder

	b.WriteString("git checkout -- . && git clean -fd")

	for _, u := range st.Untracked {
		b.WriteString(" -e " + shellWord(excludePattern(u)))
	}

	b.WriteString(" -- .")

	return b.String(), nil
}

// excludePattern is the exclude pattern git clean -e reads as the one path p, from the repository
// root: anchored with a slash, with the characters a pattern gives a meaning to escaped, trailing
// spaces too. A folder's path keeps its slash at the end.
func excludePattern(p string) string {
	var b strings.Builder

	b.WriteString("/")

	trimmed := strings.TrimRight(p, " ")

	for _, c := range trimmed {
		if strings.ContainsRune(`\*?[`, c) {
			b.WriteRune('\\')
		}

		b.WriteRune(c)
	}

	b.WriteString(strings.Repeat(`\ `, len(p)-len(trimmed)))

	return b.String()
}

// shellWord is s as one word for a POSIX shell: as it is when it is safe, else in single quotes.
func shellWord(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}

	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
