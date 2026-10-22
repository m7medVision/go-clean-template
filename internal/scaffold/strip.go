package scaffold

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/tools/imports"
)

// _tabWidth is gofmt's.
const _tabWidth = 8

// ErrBadMarker is a scaffold marker that is unbalanced, names an unknown feature, or sits in a file
// type without comments.
var ErrBadMarker = errors.New("bad scaffold marker")

// Rules decide what Strip keeps: the blocks whose features are all in Keep.
type Rules struct {
	Manifest *Manifest
	Keep     Set
}

// keeps reports whether a block tagged with ids survives: every id must be kept.
func (r Rules) keeps(ids []string) bool {
	return !slices.ContainsFunc(ids, func(id string) bool { return !r.Keep.Has(id) })
}

// comment is a file type's line-comment syntax; close is empty unless comments need closing.
type comment struct {
	open, close string
}

// commentFor picks the comment syntax of a file from its name. Files without one cannot hold markers.
func commentFor(name string) (comment, bool) {
	base := path.Base(name)

	switch {
	case base == "Makefile", base == "Dockerfile", base == "Caddyfile", strings.HasPrefix(base, ".env"),
		base == ".gitignore", base == ".dockerignore":
		return comment{open: "#"}, true
	case base == "go.mod":
		return comment{open: "//"}, true
	}

	switch path.Ext(base) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".proto", ".jsonc":
		return comment{open: "//"}, true
	case ".yml", ".yaml", ".toml", ".hcl", ".sh", ".graphqls", ".graphql", ".conf":
		return comment{open: "#"}, true
	case ".md", ".html":
		return comment{open: "<!--", close: "-->"}, true
	case ".sql":
		return comment{open: "--"}, true
	}

	return comment{}, false
}

// markers matches one comment syntax's block and trailing markers.
type markers struct {
	block, trailing *regexp.Regexp
}

func newMarkers(c comment) markers {
	closing := ""
	if c.close != "" {
		closing = regexp.QuoteMeta(c.close)
	}

	open := regexp.QuoteMeta(c.open)

	return markers{
		block:    regexp.MustCompile(`^\s*` + open + `\s*scaffold:([a-z0-9,-]+):(begin|end)\s*` + closing + `\s*$`),
		trailing: regexp.MustCompile(`^(.*\S)\s*` + open + `\s*scaffold:([a-z0-9,-]+)\s*` + closing + `\s*$`),
	}
}

// Strip removes from content the blocks and lines of features the rules drop, and the marker lines
// of those they keep. Go files then lose the imports only the dropped code used.
func (r Rules) Strip(name string, content []byte) ([]byte, error) {
	c, ok := commentFor(name)
	if !ok {
		if regexp.MustCompile(`scaffold:[a-z0-9,-]+:(begin|end)`).Match(content) {
			return nil, fmt.Errorf("%w: %s: markers need a file type with comments", ErrBadMarker, name)
		}

		return content, nil
	}

	out, changed, err := r.stripLines(name, newMarkers(c), string(content))
	if err != nil {
		return nil, err
	}

	if !changed {
		return content, nil
	}

	if path.Ext(name) == ".go" {
		fixed, err := imports.Process(name, []byte(out), &imports.Options{Comments: true, TabIndent: true, TabWidth: _tabWidth})
		if err != nil {
			return nil, fmt.Errorf("scaffold - Strip - imports.Process %s: %w", name, err)
		}

		return fixed, nil
	}

	return []byte(out), nil
}

// frame is an open block marker.
type frame struct {
	ids  string
	line int
	kept bool
}

// stripper walks one file's lines.
type stripper struct {
	rules   Rules
	name    string
	m       markers
	stack   []frame
	out     []string
	seam    bool // a removal just happened: a blank line next to another one goes too
	changed bool
}

func (r Rules) stripLines(name string, m markers, content string) (out string, changed bool, err error) {
	s := &stripper{rules: r, name: name, m: m}

	for i, line := range strings.Split(content, "\n") {
		if err := s.line(i+1, line); err != nil {
			return "", false, err
		}
	}

	if len(s.stack) > 0 {
		top := s.stack[len(s.stack)-1]

		return "", false, fmt.Errorf("%w: %s:%d: scaffold:%s:begin is never closed", ErrBadMarker, name, top.line, top.ids)
	}

	return strings.Join(s.out, "\n"), s.changed, nil
}

func (s *stripper) dropping() bool {
	return slices.ContainsFunc(s.stack, func(f frame) bool { return !f.kept })
}

func (s *stripper) line(n int, line string) error {
	if g := s.m.block.FindStringSubmatch(line); g != nil {
		s.changed = true

		return s.marker(n, g[1], g[2] == "begin")
	}

	if g := s.m.trailing.FindStringSubmatch(line); g != nil {
		ids, err := s.ids(n, g[2])
		if err != nil {
			return err
		}

		s.changed = true

		if s.dropping() || !s.rules.keeps(ids) {
			s.seam = true

			return nil
		}

		s.emit(g[1])

		return nil
	}

	if !s.dropping() {
		s.emit(line)
	}

	return nil
}

func (s *stripper) marker(n int, tag string, begin bool) error {
	ids, err := s.ids(n, tag)
	if err != nil {
		return err
	}

	if begin {
		s.stack = append(s.stack, frame{ids: tag, line: n, kept: s.rules.keeps(ids)})

		return nil
	}

	if len(s.stack) == 0 {
		return fmt.Errorf("%w: %s:%d: scaffold:%s:end without a begin", ErrBadMarker, s.name, n, tag)
	}

	top := s.stack[len(s.stack)-1]
	if top.ids != tag {
		return fmt.Errorf("%w: %s:%d: scaffold:%s:end closes scaffold:%s:begin", ErrBadMarker, s.name, n, tag, top.ids)
	}

	s.stack = s.stack[:len(s.stack)-1]

	if !top.kept {
		s.seam = true
	}

	return nil
}

func (s *stripper) ids(n int, tag string) ([]string, error) {
	ids := strings.Split(tag, ",")

	for _, id := range ids {
		if !s.rules.Manifest.Known(id) {
			return nil, fmt.Errorf("%w: %s:%d: unknown feature %q", ErrBadMarker, s.name, n, id)
		}
	}

	return ids, nil
}

// emit appends a kept line. Right after a removal, a blank line is dropped when the output already
// ends with one (or is empty), so removing a block never leaves a double blank line.
func (s *stripper) emit(line string) {
	if s.seam && strings.TrimSpace(line) == "" && (len(s.out) == 0 || strings.TrimSpace(s.out[len(s.out)-1]) == "") {
		return
	}

	s.seam = false
	s.out = append(s.out, line)
}
