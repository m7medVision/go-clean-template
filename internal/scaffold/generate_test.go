package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _generateManifest = `
module: example.com/template
self:
  - scaffold/**
features:
  - id: rest
    core: true
  - id: grpc
    paths: [grpc/**, config/grpc.go]
  - id: web
    paths: [web/**]
  - id: passkeys
    paths: [web/src/passkey/**]
`

// writeTree writes files (path → content) under a new directory and returns it with the paths.
func writeTree(t *testing.T, files map[string]string) (root string, paths []string) {
	t.Helper()

	root = t.TempDir()

	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o600))

		paths = append(paths, name)
	}

	return root, paths
}

func template(t *testing.T) (root string, paths []string) {
	t.Helper()

	return writeTree(t, map[string]string{
		"go.mod":                 "module example.com/template\n",
		"main.go":                "package main\n\nimport _ \"example.com/template/config\"\n\n// scaffold:grpc:begin\n// grpc wiring\n// scaffold:grpc:end\n",
		"config/config.go":       "package config\n",
		"config/grpc.go":         "package config\n",
		"grpc/server.go":         "package grpc\n",
		"web/src/main.ts":        "export {}\n",
		"web/src/passkey/api.ts": "export {}\n",
		"scaffold/features.yaml": "self\n",
		"bin/logo.png":           "\x89PNG\x00example.com/template",
	})
}

func generate(t *testing.T, sel scaffold.Selection, module string) (out string, report scaffold.Report) {
	t.Helper()

	m, err := scaffold.Parse([]byte(_generateManifest))
	require.NoError(t, err)

	keep, err := m.Resolve(sel)
	require.NoError(t, err)

	src, files := template(t)
	out = filepath.Join(t.TempDir(), "out")

	report, err = scaffold.Generate(t.Context(), &scaffold.Options{
		Manifest: m, Keep: keep, Source: src, Files: files, Out: out, Module: module,
	})
	require.NoError(t, err)

	return out, report
}

func read(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(b)
}

func TestGenerateDropsTheFilesOfDroppedFeatures(t *testing.T) {
	t.Parallel()

	out, report := generate(t, scaffold.Selection{Without: []string{"grpc"}}, "")

	assert.NoFileExists(t, filepath.Join(out, "grpc", "server.go"))
	assert.NoFileExists(t, filepath.Join(out, "config", "grpc.go"))
	assert.FileExists(t, filepath.Join(out, "config", "config.go"))
	assert.Equal(t, "package main\n\nimport _ \"example.com/template/config\"\n", read(t, filepath.Join(out, "main.go")))
	assert.Equal(t, 6, report.Kept)
	assert.Equal(t, 3, report.Removed, "grpc's two files and the scaffolder's own")
}

func TestGenerateKeepsAPathOwnedBySeveralFeaturesOnlyWhenAllAreKept(t *testing.T) {
	t.Parallel()

	out, _ := generate(t, scaffold.Selection{Without: []string{"passkeys"}}, "")

	assert.FileExists(t, filepath.Join(out, "web", "src", "main.ts"))
	assert.NoFileExists(t, filepath.Join(out, "web", "src", "passkey", "api.ts"))

	out, _ = generate(t, scaffold.Selection{Without: []string{"web"}}, "")

	assert.NoFileExists(t, filepath.Join(out, "web", "src", "passkey", "api.ts"))
}

func TestGenerateNeverCopiesTheScaffolder(t *testing.T) {
	t.Parallel()

	out, _ := generate(t, scaffold.Selection{}, "")

	assert.NoDirExists(t, filepath.Join(out, "scaffold"))
}

func TestGenerateRenamesTheModuleInTextFilesOnly(t *testing.T) {
	t.Parallel()

	out, _ := generate(t, scaffold.Selection{}, "github.com/acme/billing")

	assert.Equal(t, "module github.com/acme/billing\n", read(t, filepath.Join(out, "go.mod")))
	assert.Contains(t, read(t, filepath.Join(out, "main.go")), `"github.com/acme/billing/config"`)
	assert.Equal(t, "\x89PNG\x00example.com/template", read(t, filepath.Join(out, "bin", "logo.png")), "binary files are copied as they are")
}

func TestGenerateKeepsSymlinks(t *testing.T) {
	t.Parallel()

	m, err := scaffold.Parse([]byte(_generateManifest))
	require.NoError(t, err)

	keep, err := m.Resolve(scaffold.Selection{})
	require.NoError(t, err)

	src, files := writeTree(t, map[string]string{"CLAUDE.md": "rules\n"})
	require.NoError(t, os.Symlink("CLAUDE.md", filepath.Join(src, "AGENTS.md")))

	out := filepath.Join(t.TempDir(), "out")

	_, err = scaffold.Generate(t.Context(), &scaffold.Options{
		Manifest: m, Keep: keep, Source: src, Files: append(files, "AGENTS.md"), Out: out,
	})
	require.NoError(t, err)

	target, err := os.Readlink(filepath.Join(out, "AGENTS.md"))
	require.NoError(t, err)
	assert.Equal(t, "CLAUDE.md", target)
}

func TestGenerateRefusesAnOutputThatIsNotEmpty(t *testing.T) {
	t.Parallel()

	m, err := scaffold.Parse([]byte(_generateManifest))
	require.NoError(t, err)

	keep, err := m.Resolve(scaffold.Selection{})
	require.NoError(t, err)

	src, files := template(t)
	out := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(out, "stale.txt"), []byte("x"), 0o600))

	opts := scaffold.Options{Manifest: m, Keep: keep, Source: src, Files: files, Out: out}

	_, err = scaffold.Generate(t.Context(), &opts)
	require.ErrorIs(t, err, scaffold.ErrOutputExists)

	opts.Force = true
	_, err = scaffold.Generate(t.Context(), &opts)
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(out, "stale.txt"), "force replaces the old output")
}

func TestGenerateRefusesToWriteOverTheTemplate(t *testing.T) {
	t.Parallel()

	m, err := scaffold.Parse([]byte(_generateManifest))
	require.NoError(t, err)

	keep, err := m.Resolve(scaffold.Selection{})
	require.NoError(t, err)

	src, files := template(t)

	for _, out := range []string{src, filepath.Dir(src)} {
		_, err = scaffold.Generate(t.Context(), &scaffold.Options{Manifest: m, Keep: keep, Source: src, Files: files, Out: out, Force: true})
		require.ErrorIs(t, err, scaffold.ErrOutputExists, out)
	}

	assert.FileExists(t, filepath.Join(src, "go.mod"), "the template is untouched")
}
