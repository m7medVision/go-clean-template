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

func TestGlobRendersThePathAndTheContentWithTheSameData(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"internal/{{.Name.Pkg}}/{{.Name.Snake}}.go.tmpl": file("package {{.Name.Pkg}} // {{.Module}}\n"),
	}

	got, err := templates.Glob(fsys, "internal/{{.Name.Pkg}}/{{.Name.Snake}}.go.tmpl", data(t))
	require.NoError(t, err)

	assert.Equal(t, []templates.File{{
		Path:    "internal/lineitem/line_item.go",
		Content: []byte("package lineitem // example.com/app\n"),
	}}, got)
}

func TestGlobFailsOnAFieldTheDataLacks(t *testing.T) {
	t.Parallel()

	for name, fsys := range map[string]fstest.MapFS{
		"in the content": {"a.go.tmpl": file("{{.Name.Shout}}\n")},
		"in the path":    {"{{.Nope}}.go.tmpl": file("package a\n")},
		"a bad action":   {"a.go.tmpl": file("{{.Name.Go\n")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var tmpl string

			for p := range fsys {
				tmpl = p
			}

			_, err := templates.Glob(fsys, tmpl, data(t))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tmpl)
		})
	}
}

func TestDataHasTheChosenTransportsOnly(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"t.txt.tmpl": file(`{{if .Has "rest"}}rest {{end}}{{if .Has "grpc"}}grpc {{end}}{{if .Has "graphql"}}graphql{{end}}`),
	}

	got, err := templates.Glob(fsys, "t.txt.tmpl", data(t))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "rest graphql", string(got[0].Content))
}

func TestGlobRendersEveryMatchSortedAndFailsOnNone(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"x/{{.Name.Snake}}_b.go.tmpl": file("b\n"),
		"x/{{.Name.Snake}}_a.go.tmpl": file("a\n"),
		"y/{{.Name.Snake}}.go.tmpl":   file("y\n"),
	}

	got, err := templates.Glob(fsys, "x/*.tmpl", data(t))
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "x/line_item_a.go", got[0].Path)
	assert.Equal(t, "x/line_item_b.go", got[1].Path)

	_, err = templates.Glob(fsys, "z/*.tmpl", data(t))
	assert.ErrorContains(t, err, "no templates match z/*.tmpl")
}

func TestTextDropsTheFinalNewline(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{"edits/a.go.Fn.tmpl": file("{{.Name.Lower}}Events()\n")}

	got, err := templates.Text(fsys, "edits/a.go.Fn.tmpl", data(t))
	require.NoError(t, err)
	assert.Equal(t, "lineItemEvents()", got)
}

func TestEveryEmbeddedFileIsATemplateOfASet(t *testing.T) {
	t.Parallel()

	sets := map[string]int{}

	all, err := templates.Set(".")
	require.NoError(t, err)

	require.NoError(t, fs.WalkDir(all, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		set, _, _ := strings.Cut(p, "/")
		if set == "usecase" {
			kind, _, _ := strings.Cut(strings.TrimPrefix(p, "usecase/"), "/")
			set += "/" + kind
		}

		sets[set]++

		assert.True(t, strings.HasSuffix(p, ".tmpl"), "%s does not end in .tmpl", p)
		assert.Contains(t, []string{"domain", "usecase/command", "usecase/query"}, set, "%s is in no set", p)

		return nil
	}))

	assert.Len(t, sets, 3, "every set has templates")
}

func TestSetRootsTheTemplatesOfOneSet(t *testing.T) {
	t.Parallel()

	set, err := templates.Set("usecase/query")
	require.NoError(t, err)

	_, err = fs.Stat(set, "next.tmpl")
	require.NoError(t, err)
}

func usecase(t *testing.T, kebab, path string) *templates.Usecase {
	t.Helper()

	n, err := names.Parse(kebab, "")
	require.NoError(t, err)

	return &templates.Usecase{Name: n, Kind: "command", Method: "POST", Path: path}
}

func TestAUsecaseRendersWithItsDomain(t *testing.T) {
	t.Parallel()

	d := data(t)
	d.Usecase = usecase(t, "archive-line-item", "/v1/line-items/{id}/archive")

	fsys := fstest.MapFS{
		"{{.Name.Pkg}}/{{.Usecase.Name.Snake}}.go.tmpl": file(
			"{{.Usecase.Name.Go}} {{.Name.Go}}ID {{.Usecase.Summary}}: {{.Usecase.HTTPMethod}} {{.Usecase.Target \"line-item-1\"}}\n"),
	}

	got, err := templates.Glob(fsys, "{{.Name.Pkg}}/{{.Usecase.Name.Snake}}.go.tmpl", d)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "lineitem/archive_line_item.go", got[0].Path)
	assert.Equal(t, "ArchiveLineItem LineItemID Archive line item: http.MethodPost /v1/line-items/line-item-1/archive\n",
		string(got[0].Content))
}

func TestAUsecaseActsOnOneRecordWhenItsPathHoldsTheID(t *testing.T) {
	t.Parallel()

	for path, want := range map[string]bool{
		"/v1/tasks/{id}/archive": true,
		"/v1/tasks/{id}":         true,
		"/v1/tasks/count":        false,
		"/v1/tasks":              false,
	} {
		assert.Equal(t, want, usecase(t, "archive-task", path).ByID(), path)
	}
}

func TestHTTPMethodIsTheNetHTTPConstant(t *testing.T) {
	t.Parallel()

	for method, want := range map[string]string{
		"GET": "http.MethodGet", "POST": "http.MethodPost", "PUT": "http.MethodPut",
		"PATCH": "http.MethodPatch", "DELETE": "http.MethodDelete",
	} {
		u := usecase(t, "archive-task", "/v1/tasks")
		u.Method = method

		assert.Equal(t, want, u.HTTPMethod(), method)
	}
}
