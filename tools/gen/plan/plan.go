// Package plan holds everything one command will do, in memory: the files it creates and the
// insertions it makes into existing files. Nothing reaches the disk until Write, and Write
// writes either all of it or none of it.
package plan

import (
	"errors"
	"fmt"
	"go/format"
	"maps"
	"slices"
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// ErrInvalid is a planned file that no longer parses (rule R5).
var ErrInvalid = errors.New("R5 still valid")

// Plan is what one command will create and edit. Paths are relative to the repository root and
// slash-separated.
type Plan struct {
	title   string
	creates map[string][]byte
	edits   map[string]*edited
}

type edited struct {
	src        []byte
	insertions []edit.Insertion
}

// Change is one file's content before and after the plan. Old is nil for a created file.
type Change struct {
	Path string
	Old  []byte
	New  []byte
}

// New returns an empty plan; title names it in the dry run, as in "new-domain invoice".
func New(title string) *Plan {
	return &Plan{title: title, creates: map[string][]byte{}, edits: map[string]*edited{}}
}

// Title names the plan, as in "new-domain invoice".
func (p *Plan) Title() string {
	return p.title
}

// Create plans a new file.
func (p *Plan) Create(path string, content []byte) {
	p.creates[path] = content
}

// Edit plans insertions into the existing file at path, whose content is src.
func (p *Plan) Edit(path string, src []byte, ins ...edit.Insertion) {
	e, ok := p.edits[path]
	if !ok {
		e = &edited{src: src}
		p.edits[path] = e
	}

	e.insertions = append(e.insertions, ins...)
}

// Created returns the paths of the files the plan creates, sorted.
func (p *Plan) Created() []string {
	return slices.Sorted(maps.Keys(p.creates))
}

// Edited returns the paths of the files the plan edits, sorted.
func (p *Plan) Edited() []string {
	return slices.Sorted(maps.Keys(p.edits))
}

// Changes returns every file's content before and after, as Contents does, with each edited Go
// file through go/format, so a template's insertion need not get alignment right; one that breaks
// the file is ErrInvalid.
func (p *Plan) Changes() ([]Change, error) {
	changes := p.Contents()

	for i, c := range changes {
		if c.Old == nil || !strings.HasSuffix(c.Path, ".go") {
			continue
		}

		formatted, err := format.Source(c.New)
		if err != nil {
			return nil, fmt.Errorf("%w: %s after its edits: %w", ErrInvalid, c.Path, err)
		}

		changes[i].New = formatted
	}

	return changes, nil
}

// Contents returns every file's content before and after, created files first, each group sorted
// by path, with the edits made as they are: what a file declares does not depend on its layout,
// and a file the plan breaks is left for rule R5 to report, in its turn.
func (p *Plan) Contents() []Change {
	changes := make([]Change, 0, len(p.creates)+len(p.edits))

	for _, path := range p.Created() {
		changes = append(changes, Change{Path: path, New: p.creates[path]})
	}

	for _, path := range p.Edited() {
		e := p.edits[path]
		changes = append(changes, Change{Path: path, Old: e.src, New: edit.Apply(e.src, e.insertions)})
	}

	return changes
}
