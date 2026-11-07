package main

import (
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
	"github.com/m7medvision/go-clean-template/tools/gen/templates"
)

const (
	// _domainSet is the set of templates new-domain renders, templates/domain.
	_domainSet = "domain"
	// _usecaseSet holds the sets new-usecase renders, one per kind: templates/usecase/command and
	// templates/usecase/query.
	_usecaseSet = "usecase"
	// _editsDir holds, inside a set, the text of the insertions into existing files.
	_editsDir = "edits"
	// _nextTemplate is, inside a set, the checklist the command prints once it has written: what you
	// write next.
	_nextTemplate = "next.tmpl"
)

// layer is one part of what a command generates, such as the entity layer of a domain: the files
// its templates create, and the insertions it makes into files the repository has. A command lists
// its layers (domainLayers, usecaseLayers) and plans them in order.
type layer struct {
	// transport is the transport the layer belongs to, such as rest; empty for a layer every
	// domain gets.
	transport string
	// creates are fs.Glob patterns under the set: each template one matches creates one file.
	creates []string
	// edits plans the layer's insertions; nil for a layer that only creates.
	edits func(pl *planner)
}

// selectLayers returns the layers of all that run for transports: the shared ones and those of the
// transports asked for, in their order in all.
func selectLayers(all []layer, transports []string) []layer {
	var run []layer

	for _, l := range all {
		if l.transport == "" || slices.Contains(transports, l.transport) {
			run = append(run, l)
		}
	}

	return run
}

// planner is what each layer plans with: the repository it reads, the set of templates and the
// data it renders them with, and the plan it adds to.
type planner struct {
	repo rules.Repo
	// set is the folder of the templates the command renders, as in domain or usecase/command, and
	// tmpl is those templates, rooted there.
	set  string
	tmpl fs.FS
	data *templates.Data
	plan *plan.Plan
	// errs are what planning ran into: a file to edit that does not exist (R2), a place not found
	// exactly once (R4), a template that does not render. They are kept rather than returned, so
	// the dry run still shows the plan and rules.Check reports the first rule in order.
	errs []error
	// next is the checklist the command prints once the plan is written, or once a dry run passes:
	// the files you write next and the docs you update.
	next string
}

// newPlanner returns the planner of the command whose plan is called title, on the repository r:
// it renders the templates of set with data, which it fills in with the module r's go.mod
// declares. Without the module no import of the app can be written, so planning stops there.
func newPlanner(r rules.Repo, set, title string, data *templates.Data) (*planner, error) {
	module, err := modulePath(r)
	if err != nil {
		return nil, err
	}

	tmpl, err := templates.Set(set)
	if err != nil {
		return nil, err
	}

	data.Module = module

	return &planner{repo: r, set: set, tmpl: tmpl, data: data, plan: plan.New(title)}, nil
}

// run plans those of layers that run for transports, in order, then renders the set's checklist,
// <set>/next.tmpl.
func (pl *planner) run(layers []layer, transports []string) {
	for _, l := range selectLayers(layers, transports) {
		for _, pattern := range l.creates {
			pl.create(pattern)
		}

		if l.edits != nil {
			l.edits(pl)
		}
	}

	text, err := templates.Text(pl.tmpl, _nextTemplate, pl.data)
	if err != nil {
		pl.fail(err)
	}

	pl.next = text + "\n"
}

func (pl *planner) fail(err error) {
	pl.errs = append(pl.errs, err)
}

// create plans the files that the templates matching pattern, a fs.Glob pattern under the
// planner's set such as "internal/entity/*.tmpl", render to. A created Go file goes through
// formatGo.
func (pl *planner) create(pattern string) {
	files, err := templates.Glob(pl.tmpl, pattern, pl.data)
	if err != nil {
		pl.fail(err)

		return
	}

	for _, f := range files {
		content := f.Content
		if path.Ext(f.Path) == ".go" {
			content = formatGo(content)
		}

		pl.plan.Create(f.Path, content)
	}
}

// formatGo returns the Go source src as gofmt writes it: gofmt aligns columns by the longest name
// in them, so how a created file aligns depends on the domain's name. Templates are written as
// their files for widget, which a test checks.
//
// Source that does not parse, the only source go/format refuses, is returned as it is, on purpose:
// it is a created file, which rule R5 parses with every file the plan leaves, and reports with the
// rule's name and the file's path once R2 to R4 have passed, so a dry run still shows the plan.
func formatGo(src []byte) []byte {
	formatted, err := format.Source(src)
	if err != nil {
		return src
	}

	return formatted
}

// text renders <set>/edits/<name>.tmpl, the text of one insertion. name is the edited file's
// repository path and the place in it, as in "internal/entity/event.go.EventTypes"; in the path,
// the name forms are written as in a created file's, as in
// "internal/usecase/{{.Name.Pkg}}/register.go.Register".
func (pl *planner) text(name string) string {
	text, err := templates.Text(pl.tmpl, path.Join(_editsDir, name+".tmpl"), pl.data)
	if err != nil {
		pl.fail(err)
	}

	return text
}

// controllerTemplates is the folder of the domain's controllers on transport t as the templates
// name it, as in internal/controller/restapi/v1/{{.Name.Pkg}}.
func controllerTemplates(t string) string {
	return rules.ControllerDir(t, "{{.Name.Pkg}}")
}

// editFile plans the insertions that edits return into the existing file at p, parsed with parse
// (edit.ParseGo or edit.ParseProto). A file that does not exist or parse, and an edit that fails,
// go to pl.errs; the other edits are still planned.
func editFile[F any](pl *planner, p string, parse func(string, []byte) (F, error), edits ...func(F) ([]edit.Insertion, error)) {
	src, err := pl.repo.Read(p)
	if err != nil {
		pl.fail(err)

		return
	}

	parsed, err := parse(p, src)
	if err != nil {
		pl.fail(err)

		return
	}

	for _, e := range edits {
		ins, err := e(parsed)
		if err != nil {
			pl.fail(err)

			continue
		}

		if len(ins) > 0 {
			pl.plan.Edit(p, src, ins...)
		}
	}
}

// one adapts an editor that returns one insertion to editFile.
func one(in edit.Insertion, err error) ([]edit.Insertion, error) {
	if err != nil {
		return nil, err
	}

	return []edit.Insertion{in}, nil
}

// modulePath is the module path the repository's go.mod declares, which every import of the app
// starts with.
func modulePath(r rules.Repo) (string, error) {
	src, err := fs.ReadFile(r.FS, "go.mod")
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%w: there is no go.mod: is --root the repository root?", rules.ErrTargets)
	}

	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	for line := range strings.Lines(string(src)) {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "module" {
			continue
		}

		if unquoted, err := strconv.Unquote(fields[1]); err == nil {
			return unquoted, nil
		}

		return fields[1], nil
	}

	return "", fmt.Errorf("%w: go.mod declares no module", rules.ErrTargets)
}
