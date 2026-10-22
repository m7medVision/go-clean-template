package scaffold_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _commandManifest = `
module: example.com/template
self:
  - scaffold/**
features:
  - id: rest
    core: true
  - id: grpc
    paths: [grpc/**]
presets:
  - id: api
    features: []
  - id: api-grpc
    features: [grpc]
regen:
  - run: make deps
  - run: make proto-v1
    when: grpc
`

// gitTemplate is a template checked into a new git repository, with one untracked file.
func gitTemplate(t *testing.T) string {
	t.Helper()

	root, _ := writeTree(t, map[string]string{
		"scaffold/features.yaml": _commandManifest,
		"go.mod":                 "module example.com/template\n",
		"grpc/server.go":         "package grpc\n",
		"internal/app/app.go":    "package app\n",
	})

	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.md"), []byte("new\n"), 0o600))

	return root
}

func TestFindRootWalksUpToTheManifest(t *testing.T) {
	t.Parallel()

	root := gitTemplate(t)

	got, err := scaffold.FindRoot(filepath.Join(root, "internal", "app"))
	require.NoError(t, err)
	assert.Equal(t, root, got)

	_, err = scaffold.FindRoot(t.TempDir())
	require.ErrorIs(t, err, scaffold.ErrNoTemplate)
}

func TestCommandGeneratesIntoScaffoldOutAndRegenerates(t *testing.T) {
	t.Parallel()

	root := gitTemplate(t)
	run := &fakeRunner{}

	var stdout bytes.Buffer

	cmd := scaffold.Command{Source: root, Preset: "api", Module: "github.com/acme/api"}
	require.NoError(t, cmd.Run(t.Context(), &stdout, run))

	out := filepath.Join(root, "scaffold", "out", "api")
	assert.NoFileExists(t, filepath.Join(out, "grpc", "server.go"))
	assert.FileExists(t, filepath.Join(out, "notes.md"), "untracked files are part of the template")
	assert.NoDirExists(t, filepath.Join(out, "scaffold"))
	assert.Equal(t, "module github.com/acme/api\n", read(t, filepath.Join(out, "go.mod")))
	assert.Equal(t, []string{out + ": make deps"}, run.ran)
	assert.Contains(t, stdout.String(), "scaffold/out/api")
	assert.Contains(t, stdout.String(), "features: rest")
}

// A project of its own: tools inside it must not inherit the template's .gitignore, which ignores
// scaffold/out.
func TestCommandMakesTheProjectItsOwnGitRepository(t *testing.T) {
	t.Parallel()

	root := gitTemplate(t)

	cmd := scaffold.Command{Source: root, Preset: "api", NoRegen: true}
	require.NoError(t, cmd.Run(t.Context(), &bytes.Buffer{}, &fakeRunner{}))

	out := filepath.Join(root, "scaffold", "out", "api")

	top := exec.CommandContext(t.Context(), "git", "rev-parse", "--show-toplevel")
	top.Dir = out
	got, err := top.Output()
	require.NoError(t, err)

	resolved, err := filepath.EvalSymlinks(out)
	require.NoError(t, err)
	assert.Equal(t, resolved, strings.TrimSpace(string(got)))
}

func TestCommandNamesACustomSelectionAndSkipsRegenWhenAsked(t *testing.T) {
	t.Parallel()

	root := gitTemplate(t)
	run := &fakeRunner{}

	cmd := scaffold.Command{Source: root, With: []string{"grpc"}, NoRegen: true}
	require.NoError(t, cmd.Run(t.Context(), &bytes.Buffer{}, run))

	assert.FileExists(t, filepath.Join(root, "scaffold", "out", "custom", "grpc", "server.go"))
	assert.Empty(t, run.ran)
}

func TestCommandRejectsAnInvalidSelectionBeforeWriting(t *testing.T) {
	t.Parallel()

	root := gitTemplate(t)

	cmd := scaffold.Command{Source: root, Preset: "nope"}
	err := cmd.Run(t.Context(), &bytes.Buffer{}, &fakeRunner{})

	require.ErrorIs(t, err, scaffold.ErrInvalidSelection)
	assert.NoDirExists(t, filepath.Join(root, "scaffold", "out"))
}

func TestCommandCheckRunsPreCommitInTheProjectAndNamesItOnFailure(t *testing.T) {
	t.Parallel()

	root := gitTemplate(t)
	out := filepath.Join(root, "scaffold", "out", "api")

	run := &fakeRunner{}
	cmd := scaffold.Command{Source: root, Preset: "api", Check: true}
	require.NoError(t, cmd.Run(t.Context(), &bytes.Buffer{}, run))
	assert.Equal(t, []string{out + ": make deps", out + ": make pre-commit"}, run.ran)

	cmd.Force = true
	err := cmd.Run(t.Context(), &bytes.Buffer{}, &fakeRunner{failOn: "make pre-commit"})
	require.ErrorIs(t, err, errStepFailed)
	assert.Contains(t, err.Error(), "scaffold/out/api")
}

func TestCommandNamesAChangedPresetCustom(t *testing.T) {
	t.Parallel()

	root := gitTemplate(t)

	cmd := scaffold.Command{Source: root, Preset: "api", With: []string{"grpc"}}

	assert.Equal(t, filepath.Join(root, "scaffold", "out", "custom"), cmd.OutDir(root))
}

func TestCommandThenRunsACommandInTheProjectAfterRegen(t *testing.T) {
	t.Parallel()

	root := gitTemplate(t)
	out := filepath.Join(root, "scaffold", "out", "api")

	run := &fakeRunner{}
	cmd := scaffold.Command{Source: root, Preset: "api", Then: "make dev PG_PORT=5442"}
	require.NoError(t, cmd.Run(t.Context(), &bytes.Buffer{}, run))

	assert.Equal(t, []string{out + ": make deps", out + ": make dev PG_PORT=5442"}, run.ran)
}

func TestMatrixChecksEveryPresetAndSummarises(t *testing.T) {
	t.Parallel()

	root := gitTemplate(t)
	run := &fakeRunner{failOn: "make proto-v1"}

	var stdout bytes.Buffer

	err := scaffold.Matrix(t.Context(), root, &stdout, run)
	require.ErrorIs(t, err, scaffold.ErrMatrixFailed)

	assert.Contains(t, run.ran, filepath.Join(root, "scaffold", "out", "api")+": make pre-commit")
	assert.Contains(t, stdout.String(), "PASS  api")
	assert.Contains(t, stdout.String(), "FAIL  api-grpc")
}
