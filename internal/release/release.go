// Package release names the next version of the template from its Conventional Commits and
// writes its release notes. `make release-version` and `make release-notes` run it through
// cmd/release; see docs/releasing.md.
package release

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// _firstVersion is the version of the first release, when no tag exists yet.
const _firstVersion = "v0.1.0"

// _shortHash is how much of a commit hash the notes show.
const _shortHash = 7

// ErrNotAVersion is a tag that is not a vMAJOR.MINOR.PATCH release.
var ErrNotAVersion = errors.New("not a release version")

// Commit is one commit since the last release.
type Commit struct {
	Hash    string
	Subject string
	Body    string
}

// _subject is a Conventional Commit subject: type, optional (scope), optional !, then the text.
var _subject = regexp.MustCompile(`^([a-z]+)(?:\(([^)]+)\))?(!)?: (.+)$`)

// _breakingFooter is the footer that marks a breaking change in the body.
var _breakingFooter = regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE: `)

// change is a parsed commit.
type change struct {
	kind, scope, text, hash string
	breaking                bool
}

func parse(c Commit) (change, bool) {
	m := _subject.FindStringSubmatch(c.Subject)
	if m == nil {
		return change{}, false
	}

	hash := c.Hash
	if len(hash) > _shortHash {
		hash = hash[:_shortHash]
	}

	return change{
		kind: m[1], scope: m[2], text: m[4], hash: hash,
		breaking: m[3] == "!" || _breakingFooter.MatchString(c.Body),
	}, true
}

// bump is how far a set of changes moves the version.
type bump int

const (
	bumpNone bump = iota
	bumpPatch
	bumpMinor
	bumpMajor
)

func bumpOf(commits []Commit) bump {
	b := bumpNone

	for _, c := range commits {
		ch, ok := parse(c)
		if !ok {
			continue
		}

		switch {
		case ch.breaking:
			b = max(b, bumpMajor)
		case ch.kind == "feat":
			b = max(b, bumpMinor)
		case ch.kind == "fix", ch.kind == "perf":
			b = max(b, bumpPatch)
		}
	}

	return b
}

// Next is the version after last for commits, or "" when they need no release: none is a feat,
// fix, perf or breaking change. Below 1.0 a breaking change bumps the minor, not the major. With
// no last tag, the first release is v0.1.0.
func Next(last string, commits []Commit) (string, error) {
	b := bumpOf(commits)
	if b == bumpNone {
		return "", nil
	}

	if last == "" {
		return _firstVersion, nil
	}

	v, err := parseVersion(last)
	if err != nil {
		return "", err
	}

	if b == bumpMajor && v.major == 0 {
		b = bumpMinor
	}

	switch b {
	case bumpMajor:
		v = version{major: v.major + 1}
	case bumpMinor:
		v = version{major: v.major, minor: v.minor + 1}
	case bumpPatch, bumpNone:
		v.patch++
	}

	return v.String(), nil
}

// IsVersion reports whether tag is a vMAJOR.MINOR.PATCH release.
func IsVersion(tag string) bool {
	_, err := parseVersion(tag)

	return err == nil
}

// version is a vMAJOR.MINOR.PATCH release.
type version struct{ major, minor, patch int }

func (v version) String() string { return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch) }

func (v version) less(w version) bool {
	return slices.Compare([]int{v.major, v.minor, v.patch}, []int{w.major, w.minor, w.patch}) < 0
}

func parseVersion(tag string) (version, error) {
	parts := strings.Split(strings.TrimPrefix(tag, "v"), ".")
	if !strings.HasPrefix(tag, "v") || len(parts) != 3 {
		return version{}, fmt.Errorf("%w: %q", ErrNotAVersion, tag)
	}

	nums := make([]int, len(parts))

	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || (len(p) > 1 && p[0] == '0') {
			return version{}, fmt.Errorf("%w: %q", ErrNotAVersion, tag)
		}

		nums[i] = n
	}

	return version{major: nums[0], minor: nums[1], patch: nums[2]}, nil
}

// section is one heading of the notes and the changes listed under it.
type section struct {
	title string
	match func(change) bool
}

// sections are the notes' sections, in order. A change appears in each one it matches.
func sections() []section {
	return []section{
		{"Breaking changes", func(c change) bool { return c.breaking }},
		{"Features", func(c change) bool { return c.kind == "feat" }},
		{"Fixes", func(c change) bool { return c.kind == "fix" }},
		{"Performance", func(c change) bool { return c.kind == "perf" }},
	}
}

// Notes are the release notes for commits: their breaking changes, features, fixes and
// performance work, newest first as given. Other commit types are left out.
func Notes(commits []Commit) string {
	changes := make([]change, 0, len(commits))

	for _, c := range commits {
		if ch, ok := parse(c); ok {
			changes = append(changes, ch)
		}
	}

	var b strings.Builder

	for _, s := range sections() {
		var lines []string

		for i := range changes {
			if s.match(changes[i]) {
				lines = append(lines, line(&changes[i]))
			}
		}

		if len(lines) == 0 {
			continue
		}

		if b.Len() > 0 {
			b.WriteString("\n")
		}

		fmt.Fprintf(&b, "## %s\n\n%s\n", s.title, strings.Join(lines, "\n"))
	}

	return b.String()
}

func line(c *change) string {
	if c.scope != "" {
		return fmt.Sprintf("- **%s:** %s (%s)", c.scope, c.text, c.hash)
	}

	return fmt.Sprintf("- %s (%s)", c.text, c.hash)
}
