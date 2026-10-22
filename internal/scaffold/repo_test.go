package scaffold_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests run against the real template, two directories up.
const _repoRoot = "../.."

func repoManifest(t *testing.T) (m *scaffold.Manifest, files []string) {
	t.Helper()

	m, err := scaffold.Load(filepath.Join(_repoRoot, scaffold.ManifestPath))
	require.NoError(t, err)

	files, err = scaffold.ListFiles(t.Context(), _repoRoot)
	require.NoError(t, err)

	return m, files
}

func generateRepo(t *testing.T, m *scaffold.Manifest, files, without []string) string {
	t.Helper()

	keep, err := m.Resolve(scaffold.Selection{Without: without})
	require.NoError(t, err)

	out := filepath.Join(t.TempDir(), "out")

	_, err = scaffold.Generate(t.Context(), &scaffold.Options{
		Manifest: m, Keep: keep, Source: _repoRoot, Files: files, Out: out,
	})
	require.NoError(t, err)

	return out
}

// TestEveryFileIsClaimed fails for a file that is neither core, the scaffolder's, nor a feature's:
// a new top-level file or directory must be placed in scaffold/features.yaml.
func TestEveryFileIsClaimed(t *testing.T) {
	t.Parallel()

	m, files := repoManifest(t)

	for _, name := range files {
		assert.True(t, m.Claimed(name), "%s is not claimed by core, self or a feature in %s", name, scaffold.ManifestPath)
	}
}

// TestGeneratingEverythingReproducesTheTemplate keeps every feature, so the output must be the
// template minus the scaffolder, with only marker lines removed: a block lost by a marker typo
// shows up here.
func TestGeneratingEverythingReproducesTheTemplate(t *testing.T) {
	t.Parallel()

	m, files := repoManifest(t)
	out := generateRepo(t, m, files, nil)
	everything, err := m.Resolve(scaffold.Selection{})
	require.NoError(t, err)

	block := regexp.MustCompile(`(?m)^[ \t]*(//|#|--|<!--)[ \t]*scaffold:[a-z0-9,-]+:(begin|end)[ \t]*(-->)?[ \t]*\n`)
	trailing := regexp.MustCompile(`(?m)[ \t]*(//|#|--|<!--)[ \t]*scaffold:[a-z0-9,-]+[ \t]*(-->)?[ \t]*$`)

	for _, name := range files {
		dst := filepath.Join(out, filepath.FromSlash(name))

		if !m.Keeps(everything, name) {
			assert.NoFileExists(t, dst, "the scaffolder's own files never ship")

			continue
		}

		src := filepath.Join(_repoRoot, filepath.FromSlash(name))

		info, err := os.Lstat(src)
		require.NoError(t, err)

		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		want, err := os.ReadFile(src)
		require.NoError(t, err)

		// Self blocks go even here; the stripper's own tests cover them and their blank lines.
		if strings.Contains(string(want), "scaffold:"+scaffold.SelfFeature) {
			continue
		}

		got, err := os.ReadFile(dst)
		require.NoError(t, err, name)

		assert.Equal(t, trailing.ReplaceAllString(block.ReplaceAllString(string(want), ""), ""), string(got), name)
	}
}

// TestDroppedFeaturesLeaveNoTrace generates the template without each feature (and what needs it)
// and searches the result, generated code aside, for the feature's leak words.
func TestDroppedFeaturesLeaveNoTrace(t *testing.T) {
	t.Parallel()

	m, files := repoManifest(t)

	for i := range m.Features {
		f := &m.Features[i]
		if len(f.Leaks) == 0 {
			continue
		}

		t.Run(f.ID, func(t *testing.T) {
			t.Parallel()

			out := generateRepo(t, m, files, append([]string{f.ID}, m.Dependents(f.ID)...))

			var allow *regexp.Regexp
			if len(f.Allow) > 0 {
				allow = regexp.MustCompile(strings.Join(f.Allow, "|"))
			}

			for _, hit := range leaks(t, m, out, regexp.MustCompile(strings.Join(f.Leaks, "|")), allow) {
				t.Errorf("still mentions %s: %s", f.ID, hit)
			}
		})
	}
}

// leaks lists every "file:line: text" under out, generated files aside, that leak matches and allow
// does not.
func leaks(t *testing.T, m *scaffold.Manifest, out string, leak, allow *regexp.Regexp) []string {
	t.Helper()

	var names []string

	require.NoError(t, filepath.WalkDir(out, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		rel, err := filepath.Rel(out, path)
		if err == nil && !m.IsGenerated(filepath.ToSlash(rel)) {
			names = append(names, filepath.ToSlash(rel))
		}

		return err
	}))

	root, err := os.OpenRoot(out)
	require.NoError(t, err)

	defer root.Close()

	var hits []string

	for _, name := range names {
		content, err := root.ReadFile(name)
		require.NoError(t, err)

		for n, line := range strings.Split(string(content), "\n") {
			if leak.MatchString(line) && (allow == nil || !allow.MatchString(line)) {
				hits = append(hits, fmt.Sprintf("%s:%d: %s", name, n+1, strings.TrimSpace(line)))
			}
		}
	}

	return hits
}

// TestCIChecksEveryPreset keeps the CI scaffold matrix in step with the manifest's presets.
func TestCIChecksEveryPreset(t *testing.T) {
	t.Parallel()

	m, _ := repoManifest(t)

	ci, err := os.ReadFile(filepath.Join(_repoRoot, ".github", "workflows", "ci.yml"))
	require.NoError(t, err)

	line := regexp.MustCompile(`(?m)^\s+preset: \[(.*)\]\s*$`).FindSubmatch(ci)
	require.NotNil(t, line, "ci.yml has no `preset: [...]` matrix")

	want := make([]string, 0, len(m.Presets))
	for _, p := range m.Presets {
		want = append(want, p.ID)
	}

	assert.Equal(t, strings.Join(want, ", "), string(line[1]))
}

// TestGeneratedProjectsNeverMentionTheScaffolder: the self blocks and paths are gone from every
// project, even one with every feature: the scaffolder and the template's own release tooling.
func TestGeneratedProjectsNeverMentionTheScaffolder(t *testing.T) {
	t.Parallel()

	m, files := repoManifest(t)
	out := generateRepo(t, m, files, nil)

	for _, hit := range leaks(t, m, out, regexp.MustCompile(`(?i)scaffold`), nil) {
		t.Errorf("mentions the scaffolder: %s", hit)
	}

	releases := regexp.MustCompile(`release-(version|notes)|cmd/release|internal/release|releasing\.md|release\.yml|/releases\b|(?i)conventional commits`)
	for _, hit := range leaks(t, m, out, releases, nil) {
		t.Errorf("mentions the template's releases: %s", hit)
	}
}
