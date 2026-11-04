// Package templates holds every byte the generator writes, as text/template files laid out like the
// repository they render into. domain/ is the set make new-domain renders.
//
// A file the generator creates is <set>/<repo path>.tmpl. Its path is a template too, rendered with
// the same Data as its content, so domain/internal/entity/{{.Name.Snake}}.go.tmpl creates
// internal/entity/widget.go for widget. Each one reads as its file for widget, the name of the
// golden files: the generator formats the Go files it creates, since gofmt aligns columns by the
// longest name in them, but for widget formatting must change nothing.
//
// The text an edit inserts into a shared file is <set>/edits/<repo path>.<place>.tmpl, such as
// domain/edits/internal/entity/event.go.EventTypes.tmpl for the argument it adds in EventTypes. Even
// a one-line insertion has its own, so everything the generator inserts is reviewable here; only
// the import paths an edit adds are built in code, from Data.Module. A created file writes the app's
// imports as {{.Module}}/internal/…, so it follows the module go.mod declares.
//
// Three habits keep templates parsing and reading well. In Go, a brace right before an action is
// written "{ {{- .Name.Go}}", since "{{{" does not parse. Two braces that open nested composite
// literals, as in []map[string][]string{{…}}, are written {{"{{"}}, which renders them. Comments
// say "the" or "one" before a name, never "a" or "an", which would depend on how the name sounds.
package templates

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"text/template"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
)

//go:embed domain
var _templates embed.FS

const _suffix = ".tmpl"

// ErrNoTemplates is a pattern no template matches: a layer that names the wrong files.
var ErrNoTemplates = errors.New("no templates match")

// FS returns the embedded templates, every set at its root.
func FS() fs.FS {
	return _templates
}

// Data is what every template renders with, whichever set and layer it belongs to.
type Data struct {
	// Module is the app's module path, from its go.mod: the start of every import of the app.
	Module string
	// Name is the domain's name in each of its forms.
	Name names.Name
	// Transports are the transports the domain gets, such as rest, grpc and graphql.
	Transports []string
}

// Has reports whether the domain gets transport, as in {{if .Has "rest"}}.
func (d *Data) Has(transport string) bool {
	return slices.Contains(d.Transports, transport)
}

// File is one rendered template.
type File struct {
	// Template is the template's path in the FS, as in domain/internal/entity/{{.Name.Snake}}.go.tmpl.
	Template string
	// Path is the repository path the template creates, as in internal/entity/widget.go.
	Path    string
	Content []byte
}

// Render renders the template at name in fsys. The file's path is name rendered with data, without
// its first element (the set) and its .tmpl suffix.
func Render(fsys fs.FS, name string, data *Data) (File, error) {
	src, err := fs.ReadFile(fsys, name)
	if err != nil {
		return File{}, fmt.Errorf("read template: %w", err)
	}

	p, err := execute(name+" (its path)", name, data)
	if err != nil {
		return File{}, err
	}

	content, err := execute(name, string(src), data)
	if err != nil {
		return File{}, err
	}

	_, repoPath, _ := strings.Cut(strings.TrimSuffix(string(p), _suffix), "/")

	return File{Template: name, Path: repoPath, Content: content}, nil
}

// Glob renders every template in fsys that matches pattern, which fs.Glob reads, in path order.
// A pattern that matches none is ErrNoTemplates.
func Glob(fsys fs.FS, pattern string, data *Data) ([]File, error) {
	matches, err := fs.Glob(fsys, pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("%w %s", ErrNoTemplates, pattern)
	}

	files := make([]File, 0, len(matches))

	for _, m := range matches {
		f, err := Render(fsys, m, data)
		if err != nil {
			return nil, err
		}

		files = append(files, f)
	}

	return files, nil
}

// Text renders the template at name as the text of one insertion: without its final newline, which
// the file it is in ends with.
func Text(fsys fs.FS, name string, data *Data) (string, error) {
	src, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", fmt.Errorf("read template: %w", err)
	}

	out, err := execute(name, string(src), data)
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(string(out), "\n"), nil
}

// execute parses text as the template called name and renders it with data. A field the data lacks
// is an error, not an empty string.
func execute(name, text string, data *Data) ([]byte, error) {
	t, err := template.New(name).Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return nil, fmt.Errorf("render template %s: %w", name, err)
	}

	return b.Bytes(), nil
}
