// Package mailtmpl renders transactional email from the templates embedded in templates/: the HTML
// part through html/template, the subject and text part through text/template. Every template is
// parsed once, at startup.
//
// An email named <name> is two files. <name>.html.tmpl defines "content", which
// templates/layout/layout.html.tmpl wraps and which may call the partials in
// templates/layout/partials.html.tmpl. <name>.txt.tmpl defines "subject" and "text", which
// templates/layout/layout.txt.tmpl wraps. Both execute against the entity.Email whose Template
// returns <name>.
package mailtmpl

import (
	"context"
	"embed"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
	texttemplate "text/template"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/smtp"
)

//go:embed templates
var _templates embed.FS

const (
	_dir        = "templates"
	_htmlSuffix = ".html.tmpl"
	_textSuffix = ".txt.tmpl"
	_htmlLayout = "templates/layout/*.html.tmpl"
	_textLayout = "templates/layout/*.txt.tmpl"

	// The templates Render executes: the two layout roots (named after their files) and the subject.
	_htmlRoot = "layout.html.tmpl"
	_textRoot = "layout.txt.tmpl"
	_subject  = "subject"

	// _pair is the number of arguments args takes per map entry.
	_pair = 2
)

var (
	errMissingTemplate = errors.New("mail template does not define a required template")
	errArgs            = errors.New("args takes string keys, each followed by a value")
)

type compiled struct {
	html *htmltemplate.Template
	text *texttemplate.Template
}

// Renderer -.
type Renderer struct {
	templates map[string]compiled
}

// New parses every embedded template. An error means a template is broken and the binary must not
// start.
func New() (repo.MailRenderer, error) {
	r, err := load(_templates)
	if err != nil {
		return nil, err
	}

	return newTraced(r), nil
}

func load(fsys fs.FS) (*Renderer, error) {
	names, err := emailNames(fsys)
	if err != nil {
		return nil, err
	}

	r := &Renderer{templates: make(map[string]compiled, len(names))}

	for _, name := range names {
		c, err := parse(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("mailtmpl - load %s: %w", name, err)
		}

		r.templates[name] = c
	}

	return r, nil
}

// emailNames lists the emails in fsys: one per <name>.html.tmpl directly under templates/.
func emailNames(fsys fs.FS) ([]string, error) {
	paths, err := fs.Glob(fsys, path.Join(_dir, "*"+_htmlSuffix))
	if err != nil {
		return nil, fmt.Errorf("mailtmpl - emailNames: %w", err)
	}

	names := make([]string, 0, len(paths))
	for _, p := range paths {
		names = append(names, strings.TrimSuffix(path.Base(p), _htmlSuffix))
	}

	slices.Sort(names)

	return names, nil
}

func parse(fsys fs.FS, name string) (compiled, error) {
	html, err := htmltemplate.New(name).
		Option("missingkey=error").
		Funcs(htmltemplate.FuncMap{"args": args}).
		ParseFS(fsys, _htmlLayout, path.Join(_dir, name+_htmlSuffix))
	if err != nil {
		return compiled{}, fmt.Errorf("html: %w", err)
	}

	text, err := texttemplate.New(name).
		Option("missingkey=error").
		ParseFS(fsys, _textLayout, path.Join(_dir, name+_textSuffix))
	if err != nil {
		return compiled{}, fmt.Errorf("text: %w", err)
	}

	for _, required := range []string{_htmlRoot, "content"} {
		if html.Lookup(required) == nil {
			return compiled{}, fmt.Errorf("html: %w: %q", errMissingTemplate, required)
		}
	}

	for _, required := range []string{_textRoot, _subject, "text"} {
		if text.Lookup(required) == nil {
			return compiled{}, fmt.Errorf("text: %w: %q", errMissingTemplate, required)
		}
	}

	return compiled{html: html, text: text}, nil
}

// Render executes email's templates. A field the templates read but email lacks is an error here,
// which the render test catches before a send can.
func (r *Renderer) Render(_ context.Context, email entity.Email) (smtp.Message, error) {
	c, ok := r.templates[email.Template()]
	if !ok {
		return smtp.Message{}, fmt.Errorf("mailtmpl - Render: %w: %q", entity.ErrUnknownEmail, email.Template())
	}

	subject, err := execute(c.text, _subject, email)
	if err != nil {
		return smtp.Message{}, fmt.Errorf("mailtmpl - Render %s subject: %w", email.Template(), err)
	}

	text, err := execute(c.text, _textRoot, email)
	if err != nil {
		return smtp.Message{}, fmt.Errorf("mailtmpl - Render %s text: %w", email.Template(), err)
	}

	html, err := execute(c.html, _htmlRoot, email)
	if err != nil {
		return smtp.Message{}, fmt.Errorf("mailtmpl - Render %s html: %w", email.Template(), err)
	}

	return smtp.Message{
		To: email.Recipient(),
		// A subject is a header: one line, whatever the template's whitespace.
		Subject: strings.Join(strings.Fields(subject), " "),
		HTML:    html,
		Text:    text,
	}, nil
}

type executor interface {
	ExecuteTemplate(w io.Writer, name string, data any) error
}

func execute(t executor, name string, data any) (string, error) {
	var b strings.Builder
	if err := t.ExecuteTemplate(&b, name, data); err != nil {
		return "", err // the caller wraps it with the email and part
	}

	return strings.TrimSpace(b.String()) + "\n", nil
}

// args builds a map from key/value pairs, so a partial can take more than one value:
// {{template "button" (args "URL" .LoginURL "Label" "Sign in")}}.
func args(kv ...any) (map[string]any, error) {
	if len(kv)%_pair != 0 {
		return nil, fmt.Errorf("%w: got %d arguments", errArgs, len(kv))
	}

	m := make(map[string]any, len(kv)/_pair)

	for i := 0; i < len(kv); i += _pair {
		key, ok := kv[i].(string)
		if !ok {
			return nil, fmt.Errorf("%w: key %v is a %T", errArgs, kv[i], kv[i])
		}

		m[key] = kv[i+1]
	}

	return m, nil
}
