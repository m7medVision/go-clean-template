package plan

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// _context is how many unchanged lines the dry run shows around each insertion.
const _context = 3

// DryRun prints what the plan would do: each file it creates, then each insertion into an existing
// file, with the lines around it. Inserted text is shown as given; Go files are formatted after.
func (p *Plan) DryRun(w io.Writer) error {
	var b strings.Builder

	fmt.Fprintf(&b, "%s (dry run): %d files to create, %d to edit\n", p.title, len(p.creates), len(p.edits))

	if len(p.creates) > 0 {
		b.WriteString("\n")
	}

	for _, path := range p.Created() {
		fmt.Fprintf(&b, "create %s\n", path)
	}

	for _, path := range p.Edited() {
		e := p.edits[path]

		fmt.Fprintf(&b, "\nedit   %s\n", path)

		ins := slices.Clone(e.insertions)
		slices.SortStableFunc(ins, func(a, b edit.Insertion) int { return cmp.Compare(a.Offset, b.Offset) })

		for _, in := range ins {
			hunk(&b, e.src, in)
		}
	}

	_, err := io.WriteString(w, b.String())

	return err
}

// hunk prints one insertion: the line number, the lines before it, the inserted lines marked
// with +, and the lines after it. An insertion inside a line shows that line before (-) and
// after (+).
func hunk(b *strings.Builder, src []byte, in edit.Insertion) {
	lines := strings.Split(strings.TrimSuffix(string(src), "\n"), "\n")
	line := bytes.Count(src[:in.Offset], []byte("\n")) // 0-based line holding the offset
	col := in.Offset - (bytes.LastIndexByte(src[:in.Offset], '\n') + 1)

	fmt.Fprintf(b, "  @@ line %d\n", line+1)

	for _, l := range lines[max(0, line-_context):min(line, len(lines))] {
		fmt.Fprintf(b, "    %s\n", l)
	}

	after := line

	if col == 0 {
		for _, l := range strings.Split(strings.TrimSuffix(in.Text, "\n"), "\n") {
			fmt.Fprintf(b, "  + %s\n", l)
		}
	} else {
		old := lines[line]
		fmt.Fprintf(b, "  - %s\n", old)
		fmt.Fprintf(b, "  + %s\n", old[:col]+in.Text+old[col:])

		after = line + 1
	}

	for _, l := range lines[min(after, len(lines)):min(len(lines), after+_context)] {
		fmt.Fprintf(b, "    %s\n", l)
	}
}
