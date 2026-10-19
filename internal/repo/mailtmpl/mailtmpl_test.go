package mailtmpl_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo/mailtmpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtures holds every email with all its fields set. A template without one fails
// TestEveryTemplateHasAFixture, so this list is the render test's coverage.
func fixtures() map[string]entity.Email {
	return map[string]entity.Email{
		"welcome": entity.WelcomeEmail{
			To:       "john@example.com",
			Username: "johndoe",
			LoginURL: "https://example.com/login",
		},
	}
}

// zeroValues holds every email with no field set, so a template's {{else}} branches run too.
func zeroValues() map[string]entity.Email {
	return map[string]entity.Email{
		"welcome": entity.WelcomeEmail{},
	}
}

func TestEveryTemplateHasAFixture(t *testing.T) {
	t.Parallel()

	names, err := mailtmpl.TemplateNames()
	require.NoError(t, err)

	want := make([]string, 0, len(fixtures()))
	for name := range fixtures() {
		want = append(want, name)
	}

	assert.ElementsMatch(t, want, names)
}

func TestRender_MatchesGolden(t *testing.T) {
	t.Parallel()

	r, err := mailtmpl.New()
	require.NoError(t, err)

	for name, email := range fixtures() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			msg, err := r.Render(context.Background(), email)
			require.NoError(t, err)

			assert.Equal(t, email.Recipient(), msg.To)
			assert.NotEmpty(t, msg.Subject)
			assert.NotContains(t, msg.Subject, "\n")
			assertGolden(t, name+".golden.html", msg.HTML)
			assertGolden(t, name+".golden.txt", msg.Text)
		})
	}
}

func TestRender_ZeroValue(t *testing.T) {
	t.Parallel()

	r, err := mailtmpl.New()
	require.NoError(t, err)

	for name, email := range zeroValues() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := r.Render(context.Background(), email)
			require.NoError(t, err)
		})
	}
}

func TestRender_WelcomeContent(t *testing.T) {
	t.Parallel()

	r, err := mailtmpl.New()
	require.NoError(t, err)

	msg, err := r.Render(context.Background(), entity.WelcomeEmail{
		To:       "john@example.com",
		Username: "<b>johndoe</b>",
		LoginURL: "https://example.com/login",
	})
	require.NoError(t, err)

	assert.Contains(t, msg.Subject, "<b>johndoe</b>", "the subject is plain text, not HTML")
	assert.Contains(t, msg.HTML, "&lt;b&gt;johndoe&lt;/b&gt;", "the HTML escapes user data")
	assert.NotContains(t, msg.HTML, "<b>johndoe</b>")
	assert.Contains(t, msg.HTML, `href="https://example.com/login"`)
	assert.Contains(t, msg.Text, "<b>johndoe</b>", "the text part is not HTML-escaped")
	assert.Contains(t, msg.Text, "https://example.com/login")
}

type unknownEmail struct{}

func (unknownEmail) Template() string  { return "nope" }
func (unknownEmail) Recipient() string { return "john@example.com" }

func TestRender_UnknownTemplate(t *testing.T) {
	t.Parallel()

	r, err := mailtmpl.New()
	require.NoError(t, err)

	_, err = r.Render(context.Background(), unknownEmail{})
	assert.True(t, errors.Is(err, entity.ErrUnknownEmail), "got %v", err)
}

func TestRender_FieldMissingFromTheType(t *testing.T) {
	t.Parallel()

	fsys := layoutFS()
	fsys["templates/welcome.html.tmpl"] = &fstest.MapFile{Data: []byte(`{{define "content"}}{{.UserName}}{{end}}`)}
	fsys["templates/welcome.txt.tmpl"] = &fstest.MapFile{Data: []byte(`{{define "subject"}}Hi{{end}}{{define "text"}}Hi{{end}}`)}

	r, err := mailtmpl.Load(fsys)
	require.NoError(t, err)

	_, err = r.Render(context.Background(), entity.WelcomeEmail{})
	assert.ErrorContains(t, err, "UserName")
}

func TestLoad_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files map[string]string
	}{
		{"no text template", map[string]string{
			"templates/welcome.html.tmpl": `{{define "content"}}Hi{{end}}`,
		}},
		{"html does not parse", map[string]string{
			"templates/welcome.html.tmpl": `{{define "content"}}{{.Username{{end}}`,
			"templates/welcome.txt.tmpl":  `{{define "subject"}}Hi{{end}}{{define "text"}}Hi{{end}}`,
		}},
		{"text does not parse", map[string]string{
			"templates/welcome.html.tmpl": `{{define "content"}}Hi{{end}}`,
			"templates/welcome.txt.tmpl":  `{{define "subject"}}{{if}}{{end}}`,
		}},
		{"no subject", map[string]string{
			"templates/welcome.html.tmpl": `{{define "content"}}Hi{{end}}`,
			"templates/welcome.txt.tmpl":  `{{define "text"}}Hi{{end}}`,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fsys := layoutFS()
			for name, body := range tt.files {
				fsys[name] = &fstest.MapFile{Data: []byte(body)}
			}

			_, err := mailtmpl.Load(fsys)
			assert.Error(t, err)
		})
	}
}

// layoutFS returns the real layout files in a MapFS, so a test only adds the email's own templates.
func layoutFS() fstest.MapFS {
	fsys := fstest.MapFS{}

	for _, name := range []string{"layout.html.tmpl", "partials.html.tmpl", "layout.txt.tmpl"} {
		data, err := os.ReadFile(filepath.Join("templates", "layout", name))
		if err != nil {
			panic(err)
		}

		fsys["templates/layout/"+name] = &fstest.MapFile{Data: data}
	}

	return fsys
}

// assertGolden compares got with testdata/name; UPDATE_GOLDEN=1 rewrites the file instead.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name)

	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll("testdata", 0o750))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))

		return
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "run UPDATE_GOLDEN=1 go test ./internal/repo/mailtmpl/ to create it")
	assert.Equal(t, string(want), got)
}
