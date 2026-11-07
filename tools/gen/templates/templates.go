// Package templates holds every byte the generator writes, as text/template files laid out like the
// repository they render into. domain/ is the set make new-domain renders, and usecase/ the set
// make new-usecase renders. In both, .Name is the domain; a use case's own name is .Usecase.Name.
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
// <set>/next.tmpl is the checklist the command prints once it has written: the files you write next
// and the docs you update.
//
// Three habits keep templates parsing and reading well. In Go, a brace right before an action is
// written "{ {{- .Name.Go}}", since "{{{" does not parse. Two braces that open nested composite
// literals, as in []map[string][]string{{…}}, are written {{"{{"}}, which renders them. Comments
// say "the" or "one" before a name, never "a" or "an", which would depend on how the name sounds.
//
// A use case's templates are written as their files for the widget domain's archive-widget, the
// command, and count-widgets, the query: the names of their golden files.
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

//go:embed domain usecase
var _templates embed.FS

const _suffix = ".tmpl"

// errNoTemplates is a pattern no template matches: a layer that names the wrong files.
var errNoTemplates = errors.New("no templates match")

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
	// Transports are the transports the domain gets, such as rest, grpc and graphql; for a use
	// case, the transports it gets.
	Transports []string
	// Usecase is the command or query make new-usecase adds to the domain; nil for new-domain.
	Usecase *Usecase
}

// Usecase is one command or query that make new-usecase adds to an existing domain.
type Usecase struct {
	// Name is the use case's name in each of its forms, as in archive-task.
	Name names.Name
	// Kind is command or query, which is also the package its message goes in.
	Kind string
	// Method and Path are the REST operation's HTTP method and path, as in POST and
	// /v1/tasks/{id}/archive.
	Method, Path string

	// The fields below are what the layers read from the domain's own files: a hand-written
	// domain chose these names, so no name form gives them. Each layer fills in what it reads
	// before it renders.

	// Dep is the field of the domain's Deps, in register.go, that the handler is built from, and
	// DepType its type in package repo: the event store for a command, as in Store and EventStore,
	// and the read model for a query, as in Tasks and TaskReadModel.
	Dep, DepType string
	// RESTTag is the constant the domain's REST operations are tagged with, as in _tasksTag.
	RESTTag string
	// GRPCService is the service the domain's gRPC controller serves, as in TaskService, and
	// ProtoFile the .proto file that declares it, as in docs/proto/v1/task.proto: user's controller
	// serves AuthService, from auth.proto.
	GRPCService, ProtoFile string
	// ProtoPkg and ControllerPkg are the names the domain's gRPC test file imports the proto package
	// and the controller package under, as in pb and task: a generated domain's test imports its
	// controller as grpcwidget.
	ProtoPkg, ControllerPkg string
}

// ByID reports whether the use case acts on one record, named by its ID: its REST path holds
// {id}, and its message carries the record's <Domain>ID.
func (u *Usecase) ByID() bool {
	return strings.Contains(u.Path, "{id}")
}

// HTTPMethod is the net/http constant for Method, as in http.MethodPost.
func (u *Usecase) HTTPMethod() string {
	lower := strings.ToLower(u.Method)

	return "http.Method" + strings.ToUpper(lower[:1]) + lower[1:]
}

// Summary is the use case's words as a sentence, as in "Archive task".
func (u *Usecase) Summary() string {
	return strings.ToUpper(u.Name.Words[:1]) + u.Name.Words[1:]
}

// Target is Path with id in place of {id}: the URL a test sends the request to.
func (u *Usecase) Target(id string) string {
	return strings.ReplaceAll(u.Path, "{id}", id)
}

// Has reports whether the domain gets transport, as in {{if .Has "rest"}}.
func (d *Data) Has(transport string) bool {
	return slices.Contains(d.Transports, transport)
}

// File is one rendered template.
type File struct {
	// Path is the repository path the template creates, as in internal/entity/widget.go.
	Path    string
	Content []byte
}

// render renders the template at name in fsys. The file's path is name rendered with data, without
// its first element (the set) and its .tmpl suffix.
func render(fsys fs.FS, name string, data *Data) (File, error) {
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

	return File{Path: repoPath, Content: content}, nil
}

// Glob renders every template in fsys that matches pattern, which fs.Glob reads, in path order.
// A pattern that matches none is an error.
func Glob(fsys fs.FS, pattern string, data *Data) ([]File, error) {
	matches, err := fs.Glob(fsys, pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("%w %s", errNoTemplates, pattern)
	}

	files := make([]File, 0, len(matches))

	for _, m := range matches {
		f, err := render(fsys, m, data)
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
