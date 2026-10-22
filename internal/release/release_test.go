package release_test

import (
	"testing"

	"github.com/m7medvision/go-clean-template/internal/release"
	"github.com/stretchr/testify/assert"
)

func commits(subjects ...string) []release.Commit {
	out := make([]release.Commit, 0, len(subjects))
	for i, s := range subjects {
		out = append(out, release.Commit{Hash: string(rune('a'+i)) + "123456789", Subject: s})
	}

	return out
}

func TestNext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		last    string
		commits []release.Commit
		want    string
	}{
		{"the first feat is v0.1.0", "", commits("feat: add a thing"), "v0.1.0"},
		{"the first fix is v0.1.0 too", "", commits("fix: a bug"), "v0.1.0"},
		{"a fix bumps the patch", "v0.1.0", commits("fix(web): stop a loop"), "v0.1.1"},
		{"a perf bumps the patch", "v0.1.0", commits("perf: faster"), "v0.1.1"},
		{"a feat bumps the minor and resets the patch", "v0.1.3", commits("fix: x", "feat(grpc): y"), "v0.2.0"},
		{"in 0.x a breaking change bumps the minor", "v0.1.0", commits("feat!: drop REST v0"), "v0.2.0"},
		{"a breaking chore still releases", "v0.1.0", commits("chore!: require Go 1.28"), "v0.2.0"},
		{"from 1.0 a breaking change bumps the major", "v1.2.3", commits("fix(api)!: rename a field"), "v2.0.0"},
		{"from 1.0 a feat bumps the minor", "v1.2.3", commits("feat: z"), "v1.3.0"},
		{"docs, ci and deps release nothing", "v0.1.0", commits("docs: x", "ci: y", "chore(deps): bump z", "test: w"), ""},
		{"a non-conventional subject releases nothing", "v0.1.0", commits("Update README.md", "featured: no"), ""},
		{"nothing since the tag releases nothing", "v0.1.0", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := release.Next(tt.last, tt.commits)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNextReadsABreakingChangeFooter(t *testing.T) {
	t.Parallel()

	got, err := release.Next("v1.0.0", []release.Commit{{
		Hash: "abc1234567", Subject: "feat: new config", Body: "Some text.\n\nBREAKING CHANGE: PG_URL is now DATABASE_URL\n",
	}})

	assert.NoError(t, err)
	assert.Equal(t, "v2.0.0", got)
}

func TestNextRejectsALastTagThatIsNotAVersion(t *testing.T) {
	t.Parallel()

	_, err := release.Next("latest", commits("feat: x"))

	assert.ErrorIs(t, err, release.ErrNotAVersion)
}

func TestNotesGroupTheChangesThatMatter(t *testing.T) {
	t.Parallel()

	notes := release.Notes(commits(
		"feat(grpc): stream task updates",
		"fix: keep the session on refresh",
		"docs: a typo",
		"perf(sqlc): one query instead of two",
		"feat!: rename the module",
		"chore(deps): bump x",
		"ci: cache",
		"refactor: tidy",
	))

	assert.Equal(t, `## Breaking changes

- rename the module (e123456)

## Features

- **grpc:** stream task updates (a123456)
- rename the module (e123456)

## Fixes

- keep the session on refresh (b123456)

## Performance

- **sqlc:** one query instead of two (d123456)
`, notes)
}
