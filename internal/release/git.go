package release

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// State is what git says about HEAD and the releases before it.
type State struct {
	// Last is the newest release tag HEAD contains; empty before the first release.
	Last string
	// Newest is the newest release tag in the repository, which HEAD may not contain.
	Newest string
	// NewestIsAncestor reports whether HEAD contains Newest.
	NewestIsAncestor bool
	// Commits are the non-merge commits since Last, newest first.
	Commits []Commit
}

// Read collects the State of the repository at dir.
func Read(ctx context.Context, dir string) (State, error) {
	tags, err := git(ctx, dir, "tag", "--list", "v*")
	if err != nil {
		return State{}, err
	}

	merged, err := git(ctx, dir, "tag", "--list", "v*", "--merged", "HEAD")
	if err != nil {
		return State{}, err
	}

	s := State{Last: newest(merged), Newest: newest(tags)}
	s.NewestIsAncestor = s.Newest == "" || s.Newest == s.Last

	s.Commits, err = commitsSince(ctx, dir, s.Last)

	return s, err
}

// Decide is the version to release for s, or "" and the reason there is none.
func Decide(s State) (version, reason string, err error) {
	if !s.NewestIsAncestor {
		return "", s.Newest + " is not an ancestor of HEAD: a newer release already covers this commit", nil
	}

	version, err = Next(s.Last, s.Commits)
	if err != nil || version != "" {
		return version, "", err
	}

	since := s.Last
	if since == "" {
		since = "the first commit"
	}

	return "", "no feat, fix, perf or breaking commit since " + since, nil
}

// newest is the highest release version among the lines of tags.
func newest(tags string) string {
	var (
		best  version
		found string
	)

	for tag := range strings.FieldsSeq(tags) {
		v, err := parseVersion(tag)
		if err != nil {
			continue
		}

		if found == "" || best.less(v) {
			best, found = v, tag
		}
	}

	return found
}

// The fields and records of `git log` output, separators no subject or body contains.
const (
	_fieldSep  = "\x1f"
	_recordSep = "\x1e"
)

func commitsSince(ctx context.Context, dir, last string) ([]Commit, error) {
	args := []string{"log", "--no-merges", "--format=%H" + _fieldSep + "%s" + _fieldSep + "%b" + _recordSep}
	if last != "" {
		args = append(args, last+"..HEAD")
	}

	out, err := git(ctx, dir, args...)
	if err != nil {
		return nil, err
	}

	var commits []Commit

	for record := range strings.SplitSeq(out, _recordSep) {
		fields := strings.SplitN(strings.TrimLeft(record, "\n"), _fieldSep, 3) //nolint:mnd // hash, subject, body
		if len(fields) == 3 {                                                  //nolint:mnd // hash, subject, body
			commits = append(commits, Commit{Hash: fields[0], Subject: fields[1], Body: fields[2]})
		}
	}

	return commits, nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("release - git %s: %w", strings.Join(args, " "), err)
	}

	return string(out), nil
}
