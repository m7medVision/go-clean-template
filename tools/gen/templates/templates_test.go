package templates_test

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func data(t *testing.T) *templates.Data {
	t.Helper()

	n, err := names.Parse("line-item", "")
	require.NoError(t, err)

	return &templates.Data{Module: "example.com/app", Name: n, Transports: []string{"rest", "graphql"}}
}

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

func TestRenderRendersThePathAndTheContentWithTheSameData(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"domain/internal/{{.Name.Pkg}}/{{.Name.Snake}}.go.tmpl": file("package {{.Name.Pkg}} // {{.Module}}\n"),
	}

	got, err := templates.Render(fsys, "domain/internal/{{.Name.Pkg}}/{{.Name.Snake}}.go.tmpl", data(t))
	require.NoError(t, err)

	assert.Equal(t, templates.File{
		Template: "domain/internal/{{.Name.Pkg}}/{{.Name.Snake}}.go.tmpl",
		Path:     "internal/lineitem/line_item.go",
		Content:  []byte("package lineitem // example.com/app\n"),
	}, got)
}

func TestRenderFailsOnAFieldTheDataLacks(t *testing.T) {
	t.Parallel()

	for name, fsys := range map[string]fstest.MapFS{
		"in the content": {"domain/a.go.tmpl": file("{{.Name.Shout}}\n")},
		"in the path":    {"domain/{{.Nope}}.go.tmpl": file("package a\n")},
		"a bad action":   {"domain/a.go.tmpl": file("{{.Name.Go\n")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var tmpl string

			for p := range fsys {
				tmpl = p
			}

			_, err := templates.Render(fsys, tmpl, data(t))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tmpl)
		})
	}
}

func TestDataHasTheChosenTransportsOnly(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"domain/t.txt.tmpl": file(`{{if .Has "rest"}}rest {{end}}{{if .Has "grpc"}}grpc {{end}}{{if .Has "graphql"}}graphql{{end}}`),
	}

	got, err := templates.Render(fsys, "domain/t.txt.tmpl", data(t))
	require.NoError(t, err)
	assert.Equal(t, "rest graphql", string(got.Content))
}

func TestGlobRendersEveryMatchSortedAndFailsOnNone(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"domain/x/{{.Name.Snake}}_b.go.tmpl": file("b\n"),
		"domain/x/{{.Name.Snake}}_a.go.tmpl": file("a\n"),
		"domain/y/{{.Name.Snake}}.go.tmpl":   file("y\n"),
	}

	got, err := templates.Glob(fsys, "domain/x/*.tmpl", data(t))
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "x/line_item_a.go", got[0].Path)
	assert.Equal(t, "x/line_item_b.go", got[1].Path)

	_, err = templates.Glob(fsys, "domain/z/*.tmpl", data(t))
	require.ErrorIs(t, err, templates.ErrNoTemplates)
}

func TestTextDropsTheFinalNewline(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{"domain/edits/a.go.Fn.tmpl": file("{{.Name.Lower}}Events()\n")}

	got, err := templates.Text(fsys, "domain/edits/a.go.Fn.tmpl", data(t))
	require.NoError(t, err)
	assert.Equal(t, "lineItemEvents()", got)
}

func TestEveryEmbeddedFileIsATemplateOfASet(t *testing.T) {
	t.Parallel()

	require.NoError(t, fs.WalkDir(templates.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		assert.True(t, strings.HasSuffix(p, ".tmpl"), "%s does not end in .tmpl", p)
		assert.True(t, strings.HasPrefix(p, "domain/"), "%s is in no set", p)

		return nil
	}))
}
