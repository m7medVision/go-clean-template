// Package edit finds where each change to a shared file goes, by parsing the file, and returns
// the change as an insertion. A parser only locates the place: nothing here re-prints a parsed
// file, so the comments and layout around an insertion stay as they were. Nothing here writes.
package edit

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrAnchor is a change whose place was found zero times or more than once (rule R4).
var ErrAnchor = errors.New("R4 one anchor")

// Insertion is text to insert into a file at a byte offset. Every change the generator makes to
// an existing file is one: nothing is deleted or rewritten.
type Insertion struct {
	Offset int
	Text   string
}

// Apply returns src with every insertion made. Offsets refer to src. Insertions at one offset
// keep the order they are given in.
func Apply(src []byte, ins []Insertion) []byte {
	order := make([]int, len(ins))
	for i := range order {
		order[i] = i
	}

	// From the end backwards, so earlier offsets stay valid; at one offset, the last given goes in
	// first, so the first given ends up first.
	slices.SortStableFunc(order, func(a, b int) int {
		if ins[a].Offset != ins[b].Offset {
			return ins[b].Offset - ins[a].Offset
		}

		return b - a
	})

	out := slices.Clone(src)
	for _, i := range order {
		out = slices.Insert(out, ins[i].Offset, []byte(ins[i].Text)...)
	}

	return out
}

func anchorError(path, what string, found int) error {
	return fmt.Errorf("%w: %s: found %s %d times, want once", ErrAnchor, path, what, found)
}

// indent prefixes every non-empty line of text with prefix and ends it with a newline.
func indent(text, prefix string) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}

	return strings.Join(lines, "\n") + "\n"
}

// lineStart is the offset where the line holding off begins.
func lineStart(src []byte, off int) int {
	return strings.LastIndexByte(string(src[:off]), '\n') + 1
}

// nextLineStart is the offset where the line after the one holding off begins.
func nextLineStart(src []byte, off int) int {
	i := strings.IndexByte(string(src[off:]), '\n')
	if i < 0 {
		return len(src)
	}

	return off + i + 1
}

// lineIndent is the leading whitespace of the line holding off.
func lineIndent(src []byte, off int) string {
	line := src[lineStart(src, off):]

	return string(line[:len(line)-len(strings.TrimLeft(string(line), " \t"))])
}
