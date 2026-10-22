package release_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/release"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repo is a throwaway git repository with a fixed identity.
type repo struct {
	t   *testing.T
	dir string
}

func newRepo(t *testing.T) repo {
	t.Helper()

	r := repo{t: t, dir: t.TempDir()}
	r.git("init", "--quiet", "--initial-branch=master")

	return r
}

func (r repo) git(args ...string) string {
	r.t.Helper()

	cmd := exec.CommandContext(r.t.Context(), "git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(cmd.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	)

	out, err := cmd.CombinedOutput()
	require.NoError(r.t, err, "git %s: %s", strings.Join(args, " "), out)

	return strings.TrimSpace(string(out))
}

func (r repo) commit(subject string) {
	r.t.Helper()
	r.git("commit", "--quiet", "--allow-empty", "-m", subject)
}

func (r repo) decide() (version, reason string) {
	r.t.Helper()

	state, err := release.Read(r.t.Context(), r.dir)
	require.NoError(r.t, err)

	version, reason, err = release.Decide(state)
	require.NoError(r.t, err)

	return version, reason
}

func TestDecideFollowsTheHistory(t *testing.T) {
	t.Parallel()

	r := newRepo(t)

	r.commit("chore: initial")
	r.commit("feat: the first feature")

	version, _ := r.decide()
	assert.Equal(t, "v0.1.0", version, "no tag yet: the first release")

	r.git("tag", "v0.1.0")
	r.commit("docs: explain")

	version, reason := r.decide()
	assert.Empty(t, version, "only docs since v0.1.0")
	assert.Contains(t, reason, "v0.1.0")

	// A PR merged with a merge commit: its own commits count, the merge commit does not.
	r.git("switch", "--quiet", "-c", "pr")
	r.commit("fix: a bug")
	r.git("switch", "--quiet", "master")
	r.git("merge", "--quiet", "--no-ff", "-m", "Merge pull request #1 from x/pr", "pr")

	state, err := release.Read(t.Context(), r.dir)
	require.NoError(t, err)

	subjects := make([]string, 0, len(state.Commits))
	for _, c := range state.Commits {
		subjects = append(subjects, c.Subject)
	}

	assert.ElementsMatch(t, []string{"docs: explain", "fix: a bug"}, subjects)

	version, _ = r.decide()
	assert.Equal(t, "v0.1.1", version)
}

// TestDecideSkipsACommitANewerReleaseCovers is the release workflow running late for an older
// merge: the newer merge was released first and includes it.
func TestDecideSkipsACommitANewerReleaseCovers(t *testing.T) {
	t.Parallel()

	r := newRepo(t)

	r.commit("feat: one")
	older := r.git("rev-parse", "HEAD")
	r.commit("feat: two")
	r.git("tag", "v0.1.0")
	r.git("checkout", "--quiet", older)

	version, reason := r.decide()

	assert.Empty(t, version)
	assert.Contains(t, reason, "v0.1.0 is not an ancestor")
}

func TestReadIgnoresTagsThatAreNotVersions(t *testing.T) {
	t.Parallel()

	r := newRepo(t)

	r.commit("feat: one")
	r.git("tag", "v0.1.0")
	r.commit("fix: two")
	r.git("tag", "deploy-prod")
	r.git("tag", "vnext")

	state, err := release.Read(t.Context(), r.dir)
	require.NoError(t, err)

	assert.Equal(t, "v0.1.0", state.Last)
	assert.Equal(t, "v0.1.0", state.Newest)
}
