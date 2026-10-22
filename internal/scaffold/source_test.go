package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefFromVersion(t *testing.T) {
	t.Parallel()

	tests := []struct{ version, ref string }{
		{"v0.4.0", "v0.4.0"},
		{"v1.2.3-rc.1", "v1.2.3-rc.1"},
		{"v0.0.0-20261022041711-5e4fc3213fe8", "5e4fc3213fe8"},
		{"v0.4.1-0.20261022041711-5e4fc3213fe8", "5e4fc3213fe8"},
		{"v1.19.2-0.20261022042959-a8460fbfbf7c+dirty", ""},
		{"(devel)", ""},
		{"", ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.ref, scaffold.RefFromVersion(tt.version), tt.version)
	}
}

// fakeFetch records the URL it was asked for and "extracts" a template under dir.
type fakeFetch struct{ url string }

func (f *fakeFetch) fetch(_ context.Context, url, dir string) (string, error) {
	f.url = url
	root := filepath.Join(dir, "template")

	if err := os.MkdirAll(filepath.Join(root, "scaffold"), 0o755); err != nil {
		return "", err
	}

	return root, os.WriteFile(filepath.Join(root, scaffold.ManifestPath), []byte("module: x\n"), 0o600)
}

type sourceCase struct {
	name      string
	src       scaffold.Source
	local     bool
	url       string
	outBase   string
	errSubstr string
}

func sourceCases(template, elsewhere string) []sourceCase {
	return []sourceCase{
		{
			name: "an explicit ref is fetched even inside the template",
			src:  scaffold.Source{Ref: "main", Dir: template, Version: "v0.4.0"},
			url:  "https://github.com/acme/t/archive/main.tar.gz",
			// Still the checkout's scratch area, not the checkout's root.
			outBase: filepath.Join(template, "scaffold", "out"),
		},
		{
			name:  "inside the template, its own files",
			src:   scaffold.Source{Dir: filepath.Join(template, "scaffold"), Version: "v0.4.0"},
			local: true,
		},
		{
			name:    "outside it, the CLI's own version",
			src:     scaffold.Source{Dir: elsewhere, Version: "v0.4.0"},
			url:     "https://github.com/acme/t/archive/v0.4.0.tar.gz",
			outBase: elsewhere,
		},
		{
			name:    "outside it, the commit of a pseudo-version",
			src:     scaffold.Source{Dir: elsewhere, Version: "v0.0.0-20261022041711-5e4fc3213fe8"},
			url:     "https://github.com/acme/t/archive/5e4fc3213fe8.tar.gz",
			outBase: elsewhere,
		},
		{
			name:      "outside it, with no version to fetch",
			src:       scaffold.Source{Dir: elsewhere, Version: "(devel)"},
			errSubstr: "go run github.com/acme/t/cmd/create@latest",
		},
	}
}

func TestSourceOpen(t *testing.T) {
	t.Parallel()

	template := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(template, "scaffold"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(template, scaffold.ManifestPath), []byte("module: x\n"), 0o600))

	elsewhere := t.TempDir()

	for _, tt := range sourceCases(template, elsewhere) {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := &fakeFetch{}
			src := tt.src
			src.Module, src.Fetch = "github.com/acme/t", f.fetch

			opened, err := src.Open(t.Context())
			if tt.errSubstr != "" {
				require.ErrorIs(t, err, scaffold.ErrNoTemplate)
				assert.ErrorContains(t, err, tt.errSubstr)

				return
			}

			require.NoError(t, err)

			assert.Equal(t, tt.local, opened.Local)
			assert.Equal(t, tt.outBase, opened.OutBase, "where projects go by default")
			assert.Equal(t, tt.url, f.url)
			assert.FileExists(t, filepath.Join(opened.Root, scaffold.ManifestPath))

			if tt.local {
				assert.Equal(t, template, opened.Root)
			}

			require.NoError(t, opened.Close())
			assert.FileExists(t, filepath.Join(template, scaffold.ManifestPath), "Close never removes a local template")

			if !tt.local {
				assert.NoDirExists(t, opened.Root, "Close removes the fetched copy")
			}
		})
	}
}

func TestOutDirOutsideTheTemplateIsUnderOutBase(t *testing.T) {
	t.Parallel()

	cmd := scaffold.Command{Preset: "api", OutBase: "/home/me/src"}

	assert.Equal(t, "/home/me/src/api", cmd.OutDir("/tmp/template"))
	root := t.TempDir()
	assert.Equal(t, filepath.Join(root, "scaffold", "out", "api"), (&scaffold.Command{Preset: "api"}).OutDir(root))
}
